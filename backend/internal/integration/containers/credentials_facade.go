package containers

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// EnsureRegisteredCredentials seeds every profile that opts into launch-time
// credential provisioning.
func (c *Client) EnsureRegisteredCredentials(ctx context.Context, containerName string) error {
	return c.credentials.EnsureRegistered(ctx, containerName)
}

// EnsureCredentials pushes a profile's credential files into the container.
func (c *Client) EnsureCredentials(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.Ensure(ctx, containerName, spec)
}

// SyncCredentialsFromContainer pulls credentials back to the host after a
// provider rotates them inside the container.
func (c *Client) SyncCredentialsFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.SyncFromContainer(ctx, containerName, spec)
}
