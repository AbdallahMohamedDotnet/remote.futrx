package fileusersettings

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

	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
)

var _ serviceusersettings.Repository = (*Store)(nil)

type Store struct {
	root string
	mu   sync.Mutex
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "user-settings")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create user settings dir: %w", err)
	}
	return &Store{root: dir}, nil
}

func (s *Store) Get(ctx context.Context, key serviceusersettings.Key) (serviceusersettings.Settings, error) {
	select {
	case <-ctx.Done():
		return serviceusersettings.Settings{}, ctx.Err()
	default:
	}

	path, err := s.path(key)
	if err != nil {
		return serviceusersettings.Settings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return serviceusersettings.Settings{}, serviceusersettings.ErrNotFound
		}
		return serviceusersettings.Settings{}, fmt.Errorf("read user settings: %w", err)
	}

	var settings serviceusersettings.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("parse user settings: %w", err)
	}
	return settings, nil
}

func (s *Store) Save(
	ctx context.Context,
	key serviceusersettings.Key,
	settings serviceusersettings.Settings,
) (serviceusersettings.Settings, error) {
	select {
	case <-ctx.Done():
		return serviceusersettings.Settings{}, ctx.Err()
	default:
	}

	path, err := s.path(key)
	if err != nil {
		return serviceusersettings.Settings{}, err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("marshal user settings: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("create user settings dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.root, "settings-*.tmp")
	if err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("create temp user settings: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return serviceusersettings.Settings{}, fmt.Errorf("write temp user settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("close temp user settings: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return serviceusersettings.Settings{}, fmt.Errorf("replace user settings: %w", err)
	}
	return settings, nil
}

func (s *Store) path(key serviceusersettings.Key) (string, error) {
	value := strings.TrimSpace(string(key))
	if value == "" {
		return "", serviceusersettings.ErrInvalidIdentity
	}
	sum := sha256.Sum256([]byte(value))
	return filepath.Join(s.root, "sha256-"+hex.EncodeToString(sum[:])+".json"), nil
}
