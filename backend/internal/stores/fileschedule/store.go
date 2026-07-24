// Package fileschedule persists scheduled tasks as one atomically replaced
// JSON document under the Remote data directory.
package fileschedule

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	serviceschedule "github.com/futrx-com/remote.futrx.com/internal/service/schedule"
)

const fileVersion = 1

var _ serviceschedule.Repository = (*Store)(nil)

type taskFile struct {
	Version int                    `json:"version"`
	Tasks   []serviceschedule.Task `json:"tasks"`
}

type Store struct {
	dir  string
	path string

	mu    sync.RWMutex
	tasks map[serviceschedule.ID]serviceschedule.Task
}

func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "scheduled-tasks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create scheduled tasks directory: %w", err)
	}
	store := &Store{
		dir:   dir,
		path:  filepath.Join(dir, "tasks.json"),
		tasks: make(map[serviceschedule.ID]serviceschedule.Task),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) List(ctx context.Context) ([]serviceschedule.Task, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	out := make([]serviceschedule.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		out = append(out, task)
	}
	s.mu.RUnlock()
	sortTasks(out)
	return out, nil
}

func (s *Store) Create(
	ctx context.Context,
	task serviceschedule.Task,
) (serviceschedule.Task, error) {
	if err := contextError(ctx); err != nil {
		return serviceschedule.Task{}, err
	}
	if task.ID == "" {
		task.ID = newTaskID()
	}
	if !serviceschedule.ValidID(task.ID) {
		return serviceschedule.Task{}, serviceschedule.ErrInvalidID
	}
	now := time.Now().UnixMilli()
	if task.CreatedAt == 0 {
		task.CreatedAt = now
	}
	if task.UpdatedAt == 0 {
		task.UpdatedAt = task.CreatedAt
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[task.ID]; exists {
		return serviceschedule.Task{}, serviceschedule.ErrAlreadyExists
	}
	next := cloneMap(s.tasks)
	next[task.ID] = task
	if err := s.write(next); err != nil {
		return serviceschedule.Task{}, err
	}
	s.tasks = next
	return task, nil
}

func (s *Store) Get(
	ctx context.Context,
	id serviceschedule.ID,
) (serviceschedule.Task, error) {
	if err := contextError(ctx); err != nil {
		return serviceschedule.Task{}, err
	}
	if !serviceschedule.ValidID(id) {
		return serviceschedule.Task{}, serviceschedule.ErrInvalidID
	}
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists {
		return serviceschedule.Task{}, serviceschedule.ErrNotFound
	}
	return task, nil
}

func (s *Store) Update(
	ctx context.Context,
	id serviceschedule.ID,
	update func(*serviceschedule.Task) error,
) (serviceschedule.Task, error) {
	if err := contextError(ctx); err != nil {
		return serviceschedule.Task{}, err
	}
	if !serviceschedule.ValidID(id) {
		return serviceschedule.Task{}, serviceschedule.ErrInvalidID
	}
	if update == nil {
		return serviceschedule.Task{}, errors.New("scheduled task update function is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[id]
	if !exists {
		return serviceschedule.Task{}, serviceschedule.ErrNotFound
	}
	if err := update(&task); err != nil {
		return serviceschedule.Task{}, err
	}
	// IDs are immutable even if an update callback accidentally changes one.
	task.ID = id
	next := cloneMap(s.tasks)
	next[id] = task
	if err := s.write(next); err != nil {
		return serviceschedule.Task{}, err
	}
	s.tasks = next
	return task, nil
}

func (s *Store) Delete(ctx context.Context, id serviceschedule.ID) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if !serviceschedule.ValidID(id) {
		return serviceschedule.ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[id]; !exists {
		return serviceschedule.ErrNotFound
	}
	next := cloneMap(s.tasks)
	delete(next, id)
	if err := s.write(next); err != nil {
		return err
	}
	s.tasks = next
	return nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read scheduled tasks: %w", err)
	}
	var persisted taskFile
	if err := json.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("parse scheduled tasks: %w", err)
	}
	if persisted.Version != fileVersion {
		return fmt.Errorf("parse scheduled tasks: unsupported version %d", persisted.Version)
	}
	for _, task := range persisted.Tasks {
		if !serviceschedule.ValidID(task.ID) {
			return fmt.Errorf("parse scheduled tasks: %w %q", serviceschedule.ErrInvalidID, task.ID)
		}
		if _, duplicate := s.tasks[task.ID]; duplicate {
			return fmt.Errorf("parse scheduled tasks: duplicate id %q", task.ID)
		}
		s.tasks[task.ID] = task
	}
	return nil
}

func (s *Store) write(tasks map[serviceschedule.ID]serviceschedule.Task) error {
	ordered := make([]serviceschedule.Task, 0, len(tasks))
	for _, task := range tasks {
		ordered = append(ordered, task)
	}
	sortTasks(ordered)
	data, err := json.MarshalIndent(taskFile{
		Version: fileVersion,
		Tasks:   ordered,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scheduled tasks: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(s.dir, "tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary scheduled tasks file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect temporary scheduled tasks file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary scheduled tasks file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary scheduled tasks file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary scheduled tasks file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace scheduled tasks file: %w", err)
	}

	// Persist the rename itself where the platform supports directory fsync.
	if dir, err := os.Open(s.dir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func sortTasks(tasks []serviceschedule.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt == tasks[j].CreatedAt {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].CreatedAt < tasks[j].CreatedAt
	})
}

func cloneMap(
	source map[serviceschedule.ID]serviceschedule.Task,
) map[serviceschedule.ID]serviceschedule.Task {
	cloned := make(map[serviceschedule.ID]serviceschedule.Task, len(source))
	for id, task := range source {
		cloned[id] = task
	}
	return cloned
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func newTaskID() serviceschedule.ID {
	var bytes [12]byte
	if _, err := crand.Read(bytes[:]); err != nil {
		panic(fmt.Sprintf("generate scheduled task id: %v", err))
	}
	return serviceschedule.ID(hex.EncodeToString(bytes[:]))
}
