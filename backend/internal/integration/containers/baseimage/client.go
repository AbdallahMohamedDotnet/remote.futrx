// Package baseimage translates image-building operations into container
// runtime commands.
package baseimage

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
)

// Client is the command adapter for the base-image application workflow.
type Client struct {
	runner command.Runner
}

// NewClient returns a base-image runtime backed by runner.
func NewClient(runner command.Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Available() bool {
	return c.runner.Available()
}

func (c *Client) DeleteContainer(ctx context.Context, containerName string) (string, error) {
	return c.runner.Run(ctx, "delete", "--force", containerName)
}

func (c *Client) LaunchContainer(ctx context.Context, sourceImage, containerName string) (string, error) {
	return c.runner.Run(ctx, "launch", sourceImage, containerName)
}

func (c *Client) ExecuteScript(ctx context.Context, containerName, script string) (string, error) {
	return c.runner.Run(ctx, "exec", containerName, "--", "bash", "-c", script)
}

func (c *Client) StopContainer(ctx context.Context, containerName string) (string, error) {
	return c.runner.Run(ctx, "stop", containerName)
}

func (c *Client) PublishImage(
	ctx context.Context,
	containerName,
	alias,
	description string,
) (string, error) {
	return c.runner.Run(
		ctx,
		"publish",
		containerName,
		"--alias",
		alias,
		"description="+description,
	)
}
