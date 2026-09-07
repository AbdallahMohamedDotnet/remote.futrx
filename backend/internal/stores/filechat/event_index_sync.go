package filechat

import (
	"context"
	"database/sql"
	"errors"
	"os"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

type chatIndexState struct {
	indexedBytes int64
	eventOrdinal int64
	lastSeq      int64
	fileMtimeNS  int64
	prefixHash   uint64
	tailComplete bool
}

func (index *chatEventIndex) lastEventSeq(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) (int64, error) {
	state, err := index.syncChat(ctx, id, eventsPath)
	return state.lastSeq, err
}

func (index *chatEventIndex) refreshAfterAppend(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	_, err := index.syncChatWithGrowth(ctx, id, eventsPath, true)
	return err
}

func (index *chatEventIndex) refreshAfterFallback(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	_, err := index.syncChat(ctx, id, eventsPath)
	return err
}

// syncChat validates the durable snapshot against the authoritative JSONL
// file. Appends extend the committed rows; rewrites and truncations rebuild
// them. No new state becomes visible until the whole observed range commits.
func (index *chatEventIndex) syncChat(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) (chatIndexState, error) {
	return index.syncChatWithGrowth(ctx, id, eventsPath, false)
}

func (index *chatEventIndex) syncChatWithGrowth(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
	trustedAppend bool,
) (chatIndexState, error) {
	if err := index.availabilityError(); err != nil {
		return chatIndexState{}, err
	}
	if err := ctx.Err(); err != nil {
		return chatIndexState{}, err
	}
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
		(state.indexedBytes == fileSize && state.fileMtimeNS != fileMtimeNS) ||
		(state.indexedBytes < fileSize && !state.tailComplete)
	if !rebuild && !trustedAppend && state.indexedBytes < fileSize {
		matches, matchErr := chatIndexPrefixMatches(ctx, eventsPath, state)
		if matchErr != nil {
			return chatIndexState{}, matchErr
		}
		rebuild = !matches
	}
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
		state = newChatIndexState()
	}

	turn, err := readLastIndexedTurn(ctx, tx, id)
	if err != nil {
		return chatIndexState{}, err
	}
	if fileSize > state.indexedBytes {
		writer := newChatIndexWriter(ctx, tx, id, state, turn)
		state, err = writer.indexTail(eventsPath, fileSize)
		if err != nil {
			return chatIndexState{}, err
		}
	}
	state.indexedBytes = fileSize
	state.fileMtimeNS = fileMtimeNS
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chat_event_index_state
			(chat_id, indexed_bytes, event_ordinal, last_seq, file_mtime_ns,
			 prefix_hash, tail_complete)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			indexed_bytes = excluded.indexed_bytes,
			event_ordinal = excluded.event_ordinal,
			last_seq = excluded.last_seq,
			file_mtime_ns = excluded.file_mtime_ns,
			prefix_hash = excluded.prefix_hash,
			tail_complete = excluded.tail_complete`,
		id,
		state.indexedBytes,
		state.eventOrdinal,
		state.lastSeq,
		state.fileMtimeNS,
		int64(state.prefixHash),
		state.tailComplete,
	); err != nil {
		return chatIndexState{}, err
	}
	if err := tx.Commit(); err != nil {
		return chatIndexState{}, err
	}
	if err := index.restrictFiles(); err != nil {
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
	if err := index.availabilityError(); err != nil {
		return err
	}
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
	if err := index.availabilityError(); err != nil {
		return chatIndexState{}, false, err
	}
	var state chatIndexState
	var prefixHash int64
	var tailComplete int
	err := index.db.QueryRowContext(ctx, `
		SELECT indexed_bytes, event_ordinal, last_seq, file_mtime_ns,
		       prefix_hash, tail_complete
		FROM chat_event_index_state
		WHERE chat_id = ?`, id,
	).Scan(
		&state.indexedBytes,
		&state.eventOrdinal,
		&state.lastSeq,
		&state.fileMtimeNS,
		&prefixHash,
		&tailComplete,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return chatIndexState{}, false, nil
	}
	if err != nil {
		return chatIndexState{}, false, err
	}
	state.prefixHash = uint64(prefixHash)
	state.tailComplete = tailComplete != 0
	return state, true, nil
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
