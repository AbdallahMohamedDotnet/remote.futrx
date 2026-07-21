package containers

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
)

const authPushTimeout = 30 * time.Second

// credentialSynchronizer owns bidirectional credential transfer between the
// host's canonical files and their provider-defined container destinations.
type credentialSynchronizer struct {
	profiles    *profileRegistry
	files       credentialFileSynchronizer
	directories credentialDirectorySynchronizer
}

// EnsureRegisteredCredentials seeds every profile that opts into launch-time
// credential provisioning. Errors are joined so one provider does not hide
// another provider's failure.
func (c *Client) EnsureRegisteredCredentials(ctx context.Context, containerName string) error {
	return c.credentials.ensureRegistered(ctx, containerName)
}

func (s *credentialSynchronizer) ensureRegistered(ctx context.Context, containerName string) error {
	var errs []error
	for _, profile := range s.profiles.snapshot() {
		credentials := profile.Credentials
		if credentials.Empty() || !credentials.SeedOnLaunch {
			continue
		}
		if err := s.ensure(ctx, containerName, credentials); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", credentials.Name, err))
		}
	}
	return errors.Join(errs...)
}

// EnsureCredentials pushes a profile's credential files into the container.
// Each file is only pushed when its host mtime is newer than the container
// copy, so this is cheap to call on every prompt.
func (c *Client) EnsureCredentials(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.ensure(ctx, containerName, spec)
}

func (s *credentialSynchronizer) ensure(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return s.directories.ensure(ctx, containerName, spec)
	}
	return s.files.ensure(ctx, containerName, spec)
}

// SyncCredentialsFromContainer pulls credentials back to the host after a
// provider rotates them inside the container.
func (c *Client) SyncCredentialsFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.syncFromContainer(ctx, containerName, spec)
}

func (s *credentialSynchronizer) syncFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return s.directories.syncFromContainer(ctx, containerName, spec)
	}
	return s.files.syncFromContainer(ctx, containerName, spec)
}
