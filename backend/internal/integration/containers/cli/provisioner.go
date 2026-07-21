// Package cli translates agent CLI operations into LXD commands.
package cli

import (
	"context"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	servicecli "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/cli"
)

const queryTimeout = 10 * time.Second

// Client performs raw CLI queries and mutations inside project containers.
type Client struct {
	runner command.Runner
}

var _ servicecli.Runtime = (*Client)(nil)

func NewClient(runner command.Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Available() bool {
	return c.runner.Available()
}

func (c *Client) Version(ctx context.Context, containerName, binary string) (string, error) {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return c.runner.Run(quickCtx, "exec", containerName, "--", binary, "--version")
}

func (c *Client) CommandExists(ctx context.Context, containerName, commandName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := c.runner.Run(quickCtx, "exec", containerName, "--", "which", commandName)
	return err == nil
}

func (c *Client) InstallRunning(ctx context.Context, containerName, packageName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.runner.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*"+packageName)
	return err == nil && strings.TrimSpace(out) != ""
}

func (c *Client) InstallNPM(ctx context.Context, containerName, npmPackage string) (string, error) {
	return c.runner.Run(ctx, "exec", containerName, "--",
		"npm", "install", "-g", npmPackage, "--silent")
}

func (c *Client) Repair(ctx context.Context, containerName, script string) (string, error) {
	return c.runner.Run(ctx, "exec", containerName, "--", "bash", "-c", script)
}
