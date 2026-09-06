package filechat

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	_ "modernc.org/sqlite"
)

const (
	transcriptIndexFilename = "transcript-index.sqlite"
	maxEventRecordBytes     = 16 * 1024 * 1024
)

type transcriptIndex struct {
	db *sql.DB
}

type chatIndexState struct {
	indexedBytes int64
	eventOrdinal int64
	lastSeq      int64
	fileMtimeNS  int64
}

type indexedTurnState struct {
	ordinal     int64
	sourceID    string
	hasUser     bool
	hasExisting bool
}

type indexedEventLocation struct {
	offset int64
	length int64
	seq    int64
}

func newTranscriptIndex(root string) (*transcriptIndex, error) {
	databaseURL := &url.URL{
		Scheme: "file",
		Path:   filepath.Join(root, transcriptIndexFilename),
	}
	query := databaseURL.Query()
	query.Set("_busy_timeout", "5000")
	query.Set("_synchronous", "NORMAL")
	databaseURL.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, err
	}

	// WAL allows already-indexed chats to keep serving reads while another chat
	// is being backfilled. DSN options apply the connection-scoped pragmas to
	// every member of this deliberately small pool.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	for _, statement := range []string{
		"PRAGMA journal_mode = WAL",
		`CREATE TABLE IF NOT EXISTS chat_event_index_state (
			chat_id TEXT PRIMARY KEY,
			indexed_bytes INTEGER NOT NULL,
			event_ordinal INTEGER NOT NULL,
			last_seq INTEGER NOT NULL,
			file_mtime_ns INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chat_event_offsets (
			chat_id TEXT NOT NULL,
			event_ordinal INTEGER NOT NULL,
			event_seq INTEGER NOT NULL,
			byte_offset INTEGER NOT NULL,
			byte_length INTEGER NOT NULL,
			PRIMARY KEY (chat_id, event_ordinal)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS chat_event_offsets_by_seq
			ON chat_event_offsets (chat_id, event_seq)`,
		`CREATE TABLE IF NOT EXISTS chat_transcript_turns (
			chat_id TEXT NOT NULL,
			turn_ordinal INTEGER NOT NULL,
			source_turn_id TEXT NOT NULL,
			has_user INTEGER NOT NULL,
			start_seq INTEGER NOT NULL,
			end_seq INTEGER NOT NULL,
			start_offset INTEGER NOT NULL,
			end_offset INTEGER NOT NULL,
			PRIMARY KEY (chat_id, turn_ordinal)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS chat_transcript_turns_by_start_seq
			ON chat_transcript_turns (chat_id, start_seq)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize transcript index: %w", err)
		}
	}
	return &transcriptIndex{db: db}, nil
}

// syncChat incrementally indexes bytes appended since the last successful
// transaction. A missing state row or a shorter JSONL file triggers a rebuild.
func (index *transcriptIndex) syncChat(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) (chatIndexState, error) {
	info, err := os.Stat(eventsPath)
	var fileSize int64
	var fileMtimeNS int64
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return chatIndexState{}, err
		}
	} else {
		fileSize = info.Size()
		fileMtimeNS = info.ModTime().UnixNano()
	}

	state, found, err := index.readState(ctx, id)
	if err != nil {
		return chatIndexState{}, err
	}
	rebuild := !found || state.indexedBytes > fileSize ||
		(state.indexedBytes == fileSize && state.fileMtimeNS != fileMtimeNS)
	if !rebuild && state.indexedBytes == fileSize {
		return state, nil
	}

	tx, err := index.db.BeginTx(ctx, nil)
	if err != nil {
		return chatIndexState{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if rebuild {
		if err := deleteChatIndexRows(ctx, tx, id); err != nil {
			return chatIndexState{}, err
		}
		state = chatIndexState{}
	}

	turn, err := readLastIndexedTurn(ctx, tx, id)
	if err != nil {
		return chatIndexState{}, err
	}
	if fileSize > state.indexedBytes {
		if err := indexFileTail(ctx, tx, id, eventsPath, &state, &turn); err != nil {
			return chatIndexState{}, err
		}
	}
	state.indexedBytes = fileSize
	state.fileMtimeNS = fileMtimeNS
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chat_event_index_state
			(chat_id, indexed_bytes, event_ordinal, last_seq, file_mtime_ns)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			indexed_bytes = excluded.indexed_bytes,
			event_ordinal = excluded.event_ordinal,
			last_seq = excluded.last_seq,
			file_mtime_ns = excluded.file_mtime_ns`,
		id, state.indexedBytes, state.eventOrdinal, state.lastSeq, state.fileMtimeNS,
	); err != nil {
		return chatIndexState{}, err
	}
	if err := tx.Commit(); err != nil {
		return chatIndexState{}, err
	}
	return state, nil
}

func (index *transcriptIndex) rebuildChat(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	if err := index.deleteChat(ctx, id); err != nil {
		return err
	}
	_, err := index.syncChat(ctx, id, eventsPath)
	return err
}

func (index *transcriptIndex) deleteChat(ctx context.Context, id servicechat.ID) error {
	tx, err := index.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteChatIndexRows(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteChatIndexRows(ctx context.Context, tx *sql.Tx, id servicechat.ID) error {
	for _, table := range []string{
		"chat_event_offsets",
		"chat_transcript_turns",
		"chat_event_index_state",
	} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE chat_id = ?", id); err != nil {
			return err
		}
	}
	return nil
}

func (index *transcriptIndex) readState(
	ctx context.Context,
	id servicechat.ID,
) (chatIndexState, bool, error) {
	var state chatIndexState
	err := index.db.QueryRowContext(ctx, `
		SELECT indexed_bytes, event_ordinal, last_seq, file_mtime_ns
		FROM chat_event_index_state
		WHERE chat_id = ?`, id,
	).Scan(&state.indexedBytes, &state.eventOrdinal, &state.lastSeq, &state.fileMtimeNS)
	if errors.Is(err, sql.ErrNoRows) {
		return chatIndexState{}, false, nil
	}
	return state, err == nil, err
}

func readLastIndexedTurn(
	ctx context.Context,
	tx *sql.Tx,
	id servicechat.ID,
) (indexedTurnState, error) {
	var turn indexedTurnState
	err := tx.QueryRowContext(ctx, `
		SELECT turn_ordinal, source_turn_id, has_user
		FROM chat_transcript_turns
		WHERE chat_id = ?
		ORDER BY turn_ordinal DESC
		LIMIT 1`, id,
	).Scan(&turn.ordinal, &turn.sourceID, &turn.hasUser)
	if errors.Is(err, sql.ErrNoRows) {
		return indexedTurnState{}, nil
	}
	if err != nil {
		return indexedTurnState{}, err
	}
	turn.hasExisting = true
	return turn, nil
}

func indexFileTail(
	ctx context.Context,
	tx *sql.Tx,
	id servicechat.ID,
	eventsPath string,
	state *chatIndexState,
	turn *indexedTurnState,
) error {
	file, err := os.Open(eventsPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(state.indexedBytes, io.SeekStart); err != nil {
		return err
	}

	eventStatement, err := tx.PrepareContext(ctx, `
		INSERT INTO chat_event_offsets
			(chat_id, event_ordinal, event_seq, byte_offset, byte_length)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer eventStatement.Close()
	turnInsertStatement, err := tx.PrepareContext(ctx, `
		INSERT INTO chat_transcript_turns
			(chat_id, turn_ordinal, source_turn_id, has_user,
			 start_seq, end_seq, start_offset, end_offset)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer turnInsertStatement.Close()
	turnUpdateStatement, err := tx.PrepareContext(ctx, `
		UPDATE chat_transcript_turns
		SET source_turn_id = ?, has_user = ?, end_seq = ?, end_offset = ?
		WHERE chat_id = ? AND turn_ordinal = ?`)
	if err != nil {
		return err
	}
	defer turnUpdateStatement.Close()

	reader := bufio.NewReader(file)
	offset := state.indexedBytes
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, readErr := reader.ReadBytes('\n')
		if len(raw) > 0 {
			lineOffset := offset
			offset += int64(len(raw))
			line := bytes.TrimSuffix(raw, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(line) > 0 {
				state.eventOrdinal++
				if len(line) > maxEventRecordBytes {
					return fmt.Errorf("event record exceeds %d bytes", maxEventRecordBytes)
				}
				var record eventRecord
				if err := json.Unmarshal(line, &record); err == nil {
					event := record.toDomain()
					if event.Seq == 0 {
						event.Seq = state.eventOrdinal
					}
					if event.Seq > state.lastSeq {
						state.lastSeq = event.Seq
					}
					if _, err := eventStatement.ExecContext(
						ctx,
						id,
						state.eventOrdinal,
						event.Seq,
						lineOffset,
						len(raw),
					); err != nil {
						return err
					}
					if err := indexTranscriptTurn(
						ctx,
						turnInsertStatement,
						turnUpdateStatement,
						id,
						event,
						lineOffset,
						offset,
						turn,
					); err != nil {
						return err
					}
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	state.indexedBytes = offset
	return nil
}

func indexTranscriptTurn(
	ctx context.Context,
	insertStatement *sql.Stmt,
	updateStatement *sql.Stmt,
	id servicechat.ID,
	event servicechat.Event,
	startOffset int64,
	endOffset int64,
	turn *indexedTurnState,
) error {
	startsNew := !turn.hasExisting || servicechat.EventStartsTranscriptTurn(
		turn.sourceID,
		turn.hasUser,
		true,
		event,
	)
	if startsNew {
		turn.ordinal++
		turn.sourceID = event.TurnID
		turn.hasUser = event.Type == "user"
		turn.hasExisting = true
		_, err := insertStatement.ExecContext(
			ctx,
			id,
			turn.ordinal,
			turn.sourceID,
			turn.hasUser,
			event.Seq,
			event.Seq,
			startOffset,
			endOffset,
		)
		return err
	}

	if turn.sourceID == "" && event.TurnID != "" {
		turn.sourceID = event.TurnID
	}
	turn.hasUser = turn.hasUser || event.Type == "user"
	_, err := updateStatement.ExecContext(
		ctx,
		turn.sourceID,
		turn.hasUser,
		event.Seq,
		endOffset,
		id,
		turn.ordinal,
	)
	return err
}

func (index *transcriptIndex) transcriptLocations(
	ctx context.Context,
	id servicechat.ID,
	beforeSeq int64,
	turnLimit int,
) ([]indexedEventLocation, error) {
	if turnLimit <= 0 {
		turnLimit = 1
	}
	rows, err := index.db.QueryContext(ctx, `
		SELECT start_offset, end_offset
		FROM chat_transcript_turns
		WHERE chat_id = ? AND (? <= 0 OR start_seq < ?)
		ORDER BY turn_ordinal DESC
		LIMIT ?`, id, beforeSeq, beforeSeq, turnLimit+1,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var firstOffset int64 = -1
	var lastOffset int64
	for rows.Next() {
		var startOffset, endOffset int64
		if err := rows.Scan(&startOffset, &endOffset); err != nil {
			return nil, err
		}
		if firstOffset < 0 || startOffset < firstOffset {
			firstOffset = startOffset
		}
		if endOffset > lastOffset {
			lastOffset = endOffset
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if firstOffset < 0 {
		return nil, nil
	}

	return index.eventLocationsInRange(ctx, id, firstOffset, lastOffset)
}

func (index *transcriptIndex) eventPageLocations(
	ctx context.Context,
	id servicechat.ID,
	beforeSeq int64,
	limit int,
) ([]indexedEventLocation, bool, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND (? <= 0 OR event_seq < ?)
		ORDER BY event_ordinal DESC
		LIMIT ?`, id, beforeSeq, beforeSeq, limit+1,
	)
	if err != nil {
		return nil, false, err
	}
	locations, err := scanEventLocations(rows)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(locations) > limit
	if hasMore {
		locations = locations[:limit]
	}
	for left, right := 0, len(locations)-1; left < right; left, right = left+1, right-1 {
		locations[left], locations[right] = locations[right], locations[left]
	}
	return locations, hasMore, nil
}

func (index *transcriptIndex) eventLocationsAfter(
	ctx context.Context,
	id servicechat.ID,
	afterSeq int64,
) ([]indexedEventLocation, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND event_seq > ?
		ORDER BY event_ordinal`, id, afterSeq,
	)
	if err != nil {
		return nil, err
	}
	return scanEventLocations(rows)
}

func (index *transcriptIndex) eventLocationsInRange(
	ctx context.Context,
	id servicechat.ID,
	startOffset int64,
	endOffset int64,
) ([]indexedEventLocation, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND byte_offset >= ? AND byte_offset < ?
		ORDER BY event_ordinal`, id, startOffset, endOffset,
	)
	if err != nil {
		return nil, err
	}
	return scanEventLocations(rows)
}

func scanEventLocations(rows *sql.Rows) ([]indexedEventLocation, error) {
	defer rows.Close()
	locations := make([]indexedEventLocation, 0, 64)
	for rows.Next() {
		var location indexedEventLocation
		if err := rows.Scan(&location.offset, &location.length, &location.seq); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func readIndexedEvents(
	ctx context.Context,
	eventsPath string,
	locations []indexedEventLocation,
) ([]servicechat.Event, error) {
	if len(locations) == 0 {
		return nil, nil
	}
	file, err := os.Open(eventsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.Seek(locations[0].offset, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	position := locations[0].offset

	events := make([]servicechat.Event, 0, len(locations))
	for _, location := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if location.length <= 0 || location.length > maxEventRecordBytes+2 {
			return nil, fmt.Errorf("invalid indexed event length %d", location.length)
		}
		if location.offset < position {
			return nil, errors.New("indexed event offsets are out of order")
		}
		if gap := location.offset - position; gap > 0 {
			if _, err := io.CopyN(io.Discard, reader, gap); err != nil {
				return nil, err
			}
			position += gap
		}
		line := make([]byte, int(location.length))
		if _, err := io.ReadFull(reader, line); err != nil {
			return nil, err
		}
		position += location.length
		line = bytes.TrimSuffix(line, []byte{'\n'})
		line = bytes.TrimSuffix(line, []byte{'\r'})
		var record eventRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		event := record.toDomain()
		if event.Seq == 0 {
			event.Seq = location.seq
		}
		events = append(events, event)
	}
	return events, nil
}
