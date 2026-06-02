package fileprojectaccess

// File-backed storage for per-project membership lists. One JSON file per
// project at <dataDir>/projectaccess/<projectID>.json, mode 0600. The file
// holds a flat sorted list of registered emails plus an updatedAt unix-ms
// stamp. Atomic write via temp+rename.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

var _ serviceproject.AccessRepository = (*Store)(nil)

type accessFile struct {
	Members   []string `json:"members"`
	UpdatedAt int64    `json:"updatedAt"`
}

type Store struct {
	root string

	mu    sync.Mutex
	locks map[serviceproject.ID]*sync.Mutex
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "projectaccess")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create projectaccess dir: %w", err)
	}
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

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Store) loadLocked(id serviceproject.ID) (map[string]struct{}, error) {
	raw, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]struct{}{}, nil
	}
	var f accessFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse access for %s: %w", id, err)
	}
	out := make(map[string]struct{}, len(f.Members))
	for _, m := range f.Members {
		em := normalize(m)
		if em == "" {
			continue
		}
		out[em] = struct{}{}
	}
	return out, nil
}

func (s *Store) saveLocked(id serviceproject.ID, members map[string]struct{}) error {
	if len(members) == 0 {
		if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	out := accessFile{
		Members:   make([]string, 0, len(members)),
		UpdatedAt: time.Now().UnixMilli(),
	}
	for em := range members {
		out.Members = append(out.Members, em)
	}
	sort.Strings(out.Members)

	tmp, err := os.CreateTemp(s.root, "."+string(id)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(id))
}

func (s *Store) List(_ context.Context, projectID serviceproject.ID) ([]string, error) {
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	m, err := s.loadLocked(projectID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for em := range m {
		out = append(out, em)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) Add(_ context.Context, projectID serviceproject.ID, email string) error {
	em := normalize(email)
	if em == "" {
		return errors.New("empty email")
	}
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	m, err := s.loadLocked(projectID)
	if err != nil {
		return err
	}
	if _, exists := m[em]; exists {
		return nil
	}
	m[em] = struct{}{}
	return s.saveLocked(projectID, m)
}

func (s *Store) Remove(_ context.Context, projectID serviceproject.ID, email string) error {
	em := normalize(email)
	if em == "" {
		return nil
	}
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	m, err := s.loadLocked(projectID)
	if err != nil {
		return err
	}
	if _, ok := m[em]; !ok {
		return nil
	}
	delete(m, em)
	return s.saveLocked(projectID, m)
}

func (s *Store) Set(_ context.Context, projectID serviceproject.ID, emails []string) error {
	m := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		em := normalize(e)
		if em != "" {
			m[em] = struct{}{}
		}
	}
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	return s.saveLocked(projectID, m)
}

func (s *Store) Has(_ context.Context, projectID serviceproject.ID, email string) (bool, error) {
	em := normalize(email)
	if em == "" {
		return false, nil
	}
	mu := s.lock(projectID)
	mu.Lock()
	defer mu.Unlock()
	m, err := s.loadLocked(projectID)
	if err != nil {
		return false, err
	}
	_, ok := m[em]
	return ok, nil
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
