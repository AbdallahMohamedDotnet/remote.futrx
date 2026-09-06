package filechat

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	transcriptIndexFilename = "transcript-index.sqlite"
	maxEventRecordBytes     = 16 * 1024 * 1024
)

type chatEventIndex struct {
	db *sql.DB
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

func (index *chatEventIndex) close() error {
	return index.db.Close()
}
