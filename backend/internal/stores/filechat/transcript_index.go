package filechat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

type chatEventIndex struct {
	db *sql.DB
}

type chatIndexState struct {
	indexedBytes int64
	eventOrdinal int64
	lastSeq      int64
	fileMtimeNS  int64
}

func newChatEventIndex(root string) (*chatEventIndex, error) {
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
	return &chatEventIndex{db: db}, nil
}

// syncChat incrementally indexes bytes appended since the last successful
// transaction. A missing state row or a shorter JSONL file triggers a rebuild.
func (index *chatEventIndex) syncChat(
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
		writer := newChatIndexWriter(ctx, tx, id, state, turn)
		state, err = writer.indexTail(eventsPath)
		if err != nil {
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

func (index *chatEventIndex) rebuildChat(
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

func (index *chatEventIndex) deleteChat(ctx context.Context, id servicechat.ID) error {
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

func (index *chatEventIndex) readState(
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
