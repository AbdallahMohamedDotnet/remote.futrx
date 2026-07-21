package containers

import (
	"context"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func (c *Client) AgentBrowserPort() int {
	return c.browser.Port()
}

func (c *Client) EnsureAgentBrowser(ctx context.Context, containerName string) error {
	return c.browser.Ensure(ctx, containerName)
}

func (c *Client) EnsureAgentBrowserCore(ctx context.Context, containerName string) error {
	return c.browser.EnsureCore(ctx, containerName)
}

func (c *Client) EnsureAgentBrowserView(ctx context.Context, containerName string) error {
	return c.browser.EnsureView(ctx, containerName)
}

func (c *Client) StopAgentBrowser(ctx context.Context, containerName string) error {
	return c.browser.Stop(ctx, containerName)
}

func (c *Client) StopAgentBrowserView(ctx context.Context, containerName string) error {
	return c.browser.StopView(ctx, containerName)
}

func (c *Client) AgentBrowserRunning(ctx context.Context, containerName string) (bool, error) {
	return c.browser.Running(ctx, containerName)
}

func (c *Client) AgentBrowserStatus(ctx context.Context, containerName string) (serviceproject.AgentBrowserInfo, error) {
	return c.browser.Status(ctx, containerName)
}

func (c *Client) EnsureAgentBrowserLimits(ctx context.Context, containerName string) error {
	return c.browser.EnsureLimits(ctx, containerName)
}

func (c *Client) EnsureAgentBrowserMCP(ctx context.Context, containerName string) error {
	return c.browser.EnsureMCP(ctx, containerName)
}

func (c *Client) EnsureBrowserScript(ctx context.Context, containerName string) error {
	return c.browser.EnsureScript(ctx, containerName)
}

func (c *Client) EnsureBrowserSkill(ctx context.Context, containerName string) error {
	return c.browser.EnsureSkill(ctx, containerName)
}
