package containers

import (
	"context"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func (c *Client) ListListeners(ctx context.Context, containerName string) ([]serviceproject.ContainerApp, error) {
	return c.listeners.List(ctx, containerName)
}
