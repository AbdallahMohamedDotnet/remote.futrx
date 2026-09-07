package filechat

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	chatEventIndexFilename      = "transcript-index.sqlite"
	chatEventIndexSchemaVersion = 1
	chatEventIndexFileMode      = 0o600
)

var chatEventIndexSchema = []string{
	`CREATE TABLE chat_event_index_state (
		chat_id TEXT PRIMARY KEY,
		indexed_bytes INTEGER NOT NULL,
		event_ordinal INTEGER NOT NULL,
		last_seq INTEGER NOT NULL,
		file_mtime_ns INTEGER NOT NULL,
		prefix_hash INTEGER NOT NULL,
		tail_complete INTEGER NOT NULL
	)`,
	`CREATE TABLE chat_event_offsets (
		chat_id TEXT NOT NULL,
		event_ordinal INTEGER NOT NULL,
		event_seq INTEGER NOT NULL,
		byte_offset INTEGER NOT NULL,
		byte_length INTEGER NOT NULL,
		PRIMARY KEY (chat_id, event_ordinal)
	) WITHOUT ROWID`,
	`CREATE INDEX chat_event_offsets_by_seq
		ON chat_event_offsets (chat_id, event_seq)`,
	`CREATE INDEX chat_event_offsets_by_offset
		ON chat_event_offsets (chat_id, byte_offset)`,
	`CREATE TABLE chat_transcript_turns (
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
	`CREATE INDEX chat_transcript_turns_by_start_seq
		ON chat_transcript_turns (chat_id, start_seq)`,
}

type chatEventIndex struct {
	db          *sql.DB
	path        string
	unavailable error
}

func newChatEventIndex(root string) (*chatEventIndex, error) {
	path := filepath.Join(root, chatEventIndexFilename)
	index, err := openChatEventIndex(path)
	if err == nil {
		return index, nil
	}
	if !isCorruptIndexError(err) {
		return nil, err
	}
	if removeErr := removeChatEventIndexFiles(path); removeErr != nil {
		return nil, errors.Join(err, removeErr)
	}
	return openChatEventIndex(path)
}

func openChatEventIndex(path string) (*chatEventIndex, error) {
	if err := createPrivateIndexFile(path); err != nil {
		return nil, err
	}

	databaseURL := &url.URL{Scheme: "file", Path: path}
	query := databaseURL.Query()
	query.Set("_busy_timeout", "5000")
	query.Set("_defensive", "1")
	query.Set("_dqs", "0")
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_synchronous", "NORMAL")
	query.Set("_txlock", "immediate")
	query.Add("_pragma", "trusted_schema(OFF)")
	databaseURL.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, err
	}
	// WAL lets reads for already-indexed chats proceed while another chat is
	// backfilled. Every pooled connection receives the hardened DSN settings.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	index := &chatEventIndex{db: db, path: path}
	if err := index.initializeSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize chat event index: %w", err)
	}
	if err := index.restrictFiles(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure chat event index: %w", err)
	}
	return index, nil
}

func createPrivateIndexFile(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("chat event index is not a regular file: %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, chatEventIndexFileMode)
	if err != nil {
		return fmt.Errorf("open chat event index: %w", err)
	}
	closeErr := file.Close()
	if err := os.Chmod(path, chatEventIndexFileMode); err != nil {
		return fmt.Errorf("set chat event index permissions: %w", err)
	}
	return closeErr
}

func isCorruptIndexError(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code() & 0xff
	return code == sqlite3.SQLITE_CORRUPT || code == sqlite3.SQLITE_NOTADB
}

func removeChatEventIndexFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		candidate := path + suffix
		info, err := os.Lstat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to remove non-regular chat index file: %s", candidate)
		}
		if err := os.Remove(candidate); err != nil {
			return err
		}
	}
	return nil
}

func unavailableChatEventIndex(root string, err error) *chatEventIndex {
	return &chatEventIndex{
		path:        filepath.Join(root, chatEventIndexFilename),
		unavailable: err,
	}
}

func (index *chatEventIndex) availabilityError() error {
	if index == nil {
		return errors.New("chat event index is unavailable")
	}
	return index.unavailable
}

func (index *chatEventIndex) initializeSchema() error {
	var version int
	if err := index.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == chatEventIndexSchemaVersion {
		current, err := index.hasCurrentSchema()
		if err != nil {
			return err
		}
		if current {
			return nil
		}
	}

	// The index is derived state. Replacing an unknown or obsolete schema is
	// safer and simpler than migrating rows that can be rebuilt from JSONL.
	tx, err := index.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{
		"DROP TABLE IF EXISTS chat_event_offsets",
		"DROP TABLE IF EXISTS chat_transcript_turns",
		"DROP TABLE IF EXISTS chat_event_index_state",
	} {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	for _, statement := range chatEventIndexSchema {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", chatEventIndexSchemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

func (index *chatEventIndex) hasCurrentSchema() (bool, error) {
	var count int
	err := index.db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_schema
		WHERE (type = 'table' AND name IN (
			'chat_event_index_state',
			'chat_event_offsets',
			'chat_transcript_turns'
		)) OR (type = 'index' AND name IN (
			'chat_event_offsets_by_seq',
			'chat_event_offsets_by_offset',
			'chat_transcript_turns_by_start_seq'
		))
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == len(chatEventIndexSchema), nil
}

func (index *chatEventIndex) restrictFiles() error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		err := os.Chmod(index.path+suffix, chatEventIndexFileMode)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (index *chatEventIndex) close() error {
	if index == nil || index.db == nil {
		return nil
	}
	return index.db.Close()
}
