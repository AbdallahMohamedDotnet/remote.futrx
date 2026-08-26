// Package filesessions is file-backed storage for per-account session
// registry state: SecurityPreferences (single-active-session, history,
// recovery-code-alert toggles), the active session id, bounded sign-in
// history, and any pending security alert. One JSON file per account at
// <dataDir>/sessions/<sha256-hex(email)>.json, mode 0600. An account that
// has never turned on any of the three preference flags simply has no file
// - Get returns (nil, nil), which is the correct "nothing enabled" state,
// not an error. Atomic write via temp+rename, following
// fileusersettings/store.go's shape.
package filesessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
)

var _ serviceauth.SessionRegistryStore = (*Store)(nil)

type Store struct {
	root string
	mu   sync.Mutex
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	return &Store{root: dir}, nil
}

func (s *Store) Get(ctx context.Context, email string) (*serviceauth.SessionRegistryRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	path, err := s.path(email)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session registry record: %w", err)
	}
	var record serviceauth.SessionRegistryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse session registry record: %w", err)
	}
	return &record, nil
}

func (s *Store) Save(ctx context.Context, email string, record serviceauth.SessionRegistryRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	path, err := s.path(email)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session registry record: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.root, "sessions-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp session registry record: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp session registry record: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp session registry record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp session registry record: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace session registry record: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, email string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	path, err := s.path(email)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete session registry record: %w", err)
	}
	return nil
}

func (s *Store) path(email string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(email))
	if value == "" {
		return "", errors.New("email is required")
	}
	sum := sha256.Sum256([]byte(value))
	return filepath.Join(s.root, hex.EncodeToString(sum[:])+".json"), nil
}
