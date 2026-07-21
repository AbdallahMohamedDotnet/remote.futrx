package containers

import (
	"context"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func (c *Client) Inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	return c.inspector.Inspect(ctx, containerName)
}
