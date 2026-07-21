package fileprojectsecrets

// File-backed storage for per-project secrets. One JSON file per project at
// <dataDir>/projectsecrets/<id>.json, mode 0600. The file holds a flat map
// of key -> {value, updatedAt}. Write path renames a temp file into place
// for atomic replacement.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

var _ serviceproject.SecretsRepository = (*Store)(nil)

type entry struct {
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updatedAt"`
}

type Store struct {
	root string

	mu    sync.Mutex
	locks map[serviceproject.ID]*sync.Mutex
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "projectsecrets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create projectsecrets dir: %w", err)
	}
	// Ensure dir mode (MkdirAll respects umask).
	_ = os.Chmod(dir, 0o700)
	return &Store{root: dir, locks: map[serviceproject.ID]*sync.Mutex{}}, nil
}

func (s *Store) path(id serviceproject.ID) string {
	return filepath.Join(s.root, string(id)+".json")
}

func (s *Store) lock(id serviceproject.ID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.locks[id]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.locks[id] = m
	return m
}

func (s *Store) loadLocked(id serviceproject.ID) (map[string]entry, error) {
	raw, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]entry{}, nil
		}
		return nil, err
	}
	out := map[string]entry{}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal secrets for %s: %w", id, err)
	}
	return out, nil
}

func (s *Store) saveLocked(id serviceproject.ID, m map[string]entry) error {
	tmp, err := os.CreateTemp(s.root, "."+string(id)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path(id))
}

func (s *Store) List(_ context.Context, projectID serviceproject.ID) ([]serviceproject.Secret, error) {
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()

	m, err := s.loadLocked(projectID)
	if err != nil {
		return nil, err
	}
	out := make([]serviceproject.Secret, 0, len(m))
	for k, v := range m {
		out = append(out, serviceproject.Secret{Key: k, Value: v.Value, UpdatedAt: v.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *Store) Set(_ context.Context, projectID serviceproject.ID, key, value string) (serviceproject.Secret, error) {
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()

	m, err := s.loadLocked(projectID)
	if err != nil {
		return serviceproject.Secret{}, err
	}
	now := time.Now().Unix()
	m[key] = entry{Value: value, UpdatedAt: now}
	if err := s.saveLocked(projectID, m); err != nil {
		return serviceproject.Secret{}, err
	}
	return serviceproject.Secret{Key: key, Value: value, UpdatedAt: now}, nil
}

func (s *Store) Delete(_ context.Context, projectID serviceproject.ID, key string) error {
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()

	m, err := s.loadLocked(projectID)
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	if len(m) == 0 {
		// Optional: remove the file entirely when empty. Keeps the
		// projectsecrets dir tidy.
		if err := os.Remove(s.path(projectID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return s.saveLocked(projectID, m)
}

func (s *Store) DeleteAll(_ context.Context, projectID serviceproject.ID) error {
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	if err := os.Remove(s.path(projectID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
