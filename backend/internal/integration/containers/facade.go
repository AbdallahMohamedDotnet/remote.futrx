package containers

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	containerbaseimage "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/baseimage"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	BaseImageAlias       = containerbaseimage.Alias
	BaseImageSourceImage = containerbaseimage.SourceImage
)

func (c *Client) BuildBaseImage(ctx context.Context, alias string) error {
	return c.images.Build(ctx, alias)
}

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

func (c *Client) EnsureCLI(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	return c.clis.Ensure(ctx, containerName, spec)
}

func (c *Client) EnsureCodeServer(ctx context.Context, containerName, displayName string) error {
	return c.codeServer.Ensure(ctx, containerName, displayName)
}

func (c *Client) EnsureRegisteredCredentials(ctx context.Context, containerName string) error {
	return c.credentials.EnsureRegistered(ctx, containerName)
}

func (c *Client) EnsureCredentials(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.Ensure(ctx, containerName, spec)
}

func (c *Client) SyncCredentialsFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	return c.credentials.SyncFromContainer(ctx, containerName, spec)
}

func (c *Client) ApplyContainerEnvDiff(
	ctx context.Context,
	container string,
	set map[string]string,
	unset []string,
) error {
	return c.environment.ApplyDiff(ctx, container, set, unset)
}

func (c *Client) Inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	return c.inspector.Inspect(ctx, containerName)
}

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

func (c *Client) ListListeners(ctx context.Context, containerName string) ([]serviceproject.ContainerApp, error) {
	return c.listeners.List(ctx, containerName)
}

func (c *Client) RepairNetwork(ctx context.Context, containerName string) error {
	return c.network.Repair(ctx, containerName)
}

func (c *Client) EnsureAgentInstructions(ctx context.Context, containerName string) error {
	return c.workspace.EnsureAgentInstructions(ctx, containerName)
}

func (c *Client) EnsureWorkspaceSkillLinks(ctx context.Context, containerName string) error {
	return c.workspace.EnsureSkillLinks(ctx, containerName)
}
