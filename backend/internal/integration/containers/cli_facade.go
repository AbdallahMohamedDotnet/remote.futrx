package containers

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

func (c *Client) EnsureCLI(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	return c.clis.Ensure(ctx, containerName, spec)
}
