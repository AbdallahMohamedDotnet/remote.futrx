// Package credentials synchronizes provider-owned authentication data between
// the host and project containers.
package credentials

// Credential orchestration. The host holds the canonical copy of each
// profile's files; the client pushes them into containers
// before use and pulls any rotations back afterwards so the host stays
// current. Provider packages own every path and credential rule.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
)

const (
	authPushTimeout = 30 * time.Second
	queryTimeout    = 10 * time.Second
)

// Synchronizer owns bidirectional credential transfer between the
// host's canonical files and their provider-defined container destinations.
type Synchronizer struct {
	profiles    serviceprofiles.Source
	files       fileSynchronizer
	directories directorySynchronizer
}

// NewSynchronizer returns a credential synchronizer backed by the shared
// profile registry.
func NewSynchronizer(runner command.Runner, profileSource serviceprofiles.Source) *Synchronizer {
	synchronizer := &Synchronizer{
		profiles: profileSource,
		files:    fileSynchronizer{runner: runner},
	}
	synchronizer.directories = directorySynchronizer{
		runner: runner,
		files:  &synchronizer.files,
	}
	return synchronizer
}

// EnsureRegistered seeds every profile that opts into launch-time credential
// provisioning. Errors are joined so one provider does not hide another
// provider's failure.
func (s *Synchronizer) EnsureRegistered(ctx context.Context, containerName string) error {
	var errs []error
	for _, profile := range s.profiles.Snapshot() {
		credentials := profile.Credentials
		if credentials.Empty() || !credentials.SeedOnLaunch {
			continue
		}
		if err := s.Ensure(ctx, containerName, credentials); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", credentials.Name, err))
		}
	}
	return errors.Join(errs...)
}

// Ensure pushes a profile's credential files into the container.
// Each file is only pushed when its host mtime is newer than the container
// copy, so this is cheap to call on every prompt.
func (s *Synchronizer) Ensure(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return s.directories.ensure(ctx, containerName, spec)
	}
	return s.files.ensure(ctx, containerName, spec)
}

// SyncFromContainer pulls credentials back to the host after a
// provider rotates them inside the container.
func (s *Synchronizer) SyncFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return s.directories.syncFromContainer(ctx, containerName, spec)
	}
	return s.files.syncFromContainer(ctx, containerName, spec)
}
