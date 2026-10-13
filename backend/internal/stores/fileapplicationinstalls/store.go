// Package fileapplicationinstalls persists the latest observed install per scope.
package fileapplicationinstalls

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
)

type Store struct {
	root string
	mu   sync.Mutex
}

func New(dataDir string) *Store { return &Store{root: filepath.Join(dataDir, "application-installs")} }

func (s *Store) filename(scope svc.Scope, project, app string) string {
	key, _ := json.Marshal([]string{string(scope), project, app})
	return filepath.Join(s.root, fmt.Sprintf("%x.json", sha256.Sum256(key)))
}

func (s *Store) Get(ctx context.Context, scope svc.Scope, project, app string) (svc.InstallRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return svc.InstallRun{}, false, err
	}
	data, err := os.ReadFile(s.filename(scope, project, app))
	if errors.Is(err, os.ErrNotExist) {
		return svc.InstallRun{}, false, nil
	}
	if err != nil {
		return svc.InstallRun{}, false, err
	}
	var run svc.InstallRun
	err = json.Unmarshal(data, &run)
	return run, err == nil, err
}

func (s *Store) Put(ctx context.Context, run svc.InstallRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.root, "install-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.filename(run.Scope, run.ProjectID, run.ApplicationID))
}
