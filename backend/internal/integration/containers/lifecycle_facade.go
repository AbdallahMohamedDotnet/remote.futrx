package containers

import (
	"context"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func (c *Client) Launch(ctx context.Context, project serviceproject.Meta) error {
	return c.lifecycle.Launch(ctx, project)
}

func (c *Client) EnsureBootAutostart(ctx context.Context, containerName string) error {
	return c.lifecycle.EnsureBootAutostart(ctx, containerName)
}

func (c *Client) Start(ctx context.Context, containerName string) error {
	return c.lifecycle.Start(ctx, containerName)
}

func (c *Client) Stop(ctx context.Context, containerName string) error {
	return c.lifecycle.Stop(ctx, containerName)
}

func (c *Client) Delete(ctx context.Context, containerName string) error {
	return c.lifecycle.Delete(ctx, containerName)
}

func (c *Client) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	return c.lifecycle.State(ctx, containerName)
}
