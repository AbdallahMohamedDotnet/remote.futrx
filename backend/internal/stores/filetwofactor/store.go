// Package filetwofactor is file-backed storage for per-account TOTP
// enrollment. One JSON file per enrolled account at
// <dataDir>/twofactor/<sha256-hex(email)>.json, mode 0600. An account that
// has never enrolled simply has no file - Get returns (nil, nil), which is
// the correct "2FA not enabled" state, not an error. Atomic write via
// temp+rename, following fileusersettings/store.go's shape.
package filetwofactor

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

var _ serviceauth.TwoFactorStore = (*Store)(nil)

type Store struct {
	root string
	mu   sync.Mutex
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "twofactor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create twofactor dir: %w", err)
	}
	return &Store{root: dir}, nil
}

func (s *Store) Get(ctx context.Context, email string) (*serviceauth.TwoFactorRecord, error) {
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
		return nil, fmt.Errorf("read two-factor record: %w", err)
	}
	var record serviceauth.TwoFactorRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse two-factor record: %w", err)
	}
	return &record, nil
}

func (s *Store) Save(ctx context.Context, email string, record serviceauth.TwoFactorRecord) error {
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
		return fmt.Errorf("marshal two-factor record: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create twofactor dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.root, "twofactor-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp two-factor record: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp two-factor record: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp two-factor record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp two-factor record: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace two-factor record: %w", err)
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
		return fmt.Errorf("delete two-factor record: %w", err)
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
