package containers

// Agent Browser provisioning: brings up a real headed Google Chrome inside
// the project container, rendered on a virtual display (Xvfb) and exposed two
// ways onto the SAME session - a noVNC web view the user logs in through, and
// a loopback CDP port the agent drives. The launcher script (templates/
// gui-up.sh) is workspace-resident so it survives container deletes; the host
// re-pushes it whenever the embedded template changes (sha256 marker, same
// pattern as browser.mjs / AGENTS.md).

import (
	"context"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	// AgentBrowserVNCPort is the in-container port the noVNC/websockify front
	// listens on. It is the only externally-reachable port of the GUI stack
	// and is surfaced to the user through the existing dev-URL proxy.
	AgentBrowserVNCPort = 6080
)

// agentBrowser owns installation, workspace templates, and the split core/view
// runtime lifecycle.
type agentBrowser struct {
	provisioner agentBrowserProvisioner
	runtime     agentBrowserRuntime
}

// AgentBrowserPort returns the in-container noVNC port the stack listens on.
func (c *Client) AgentBrowserPort() int { return AgentBrowserVNCPort }

// EnsureAgentBrowser starts the full stack: browser core plus noVNC view.
func (c *Client) EnsureAgentBrowser(ctx context.Context, containerName string) error {
	return c.browser.start(ctx, containerName, "start", "start agent browser")
}

// EnsureAgentBrowserCore starts only Xvfb, openbox, headed Chromium, and CDP.
func (c *Client) EnsureAgentBrowserCore(ctx context.Context, containerName string) error {
	return c.browser.start(ctx, containerName, "start-core", "start agent browser core")
}

// EnsureAgentBrowserView starts the noVNC/VNC layer on top of the same core.
func (c *Client) EnsureAgentBrowserView(ctx context.Context, containerName string) error {
	return c.browser.start(ctx, containerName, "start-view", "start agent browser view")
}

func (b *agentBrowser) start(ctx context.Context, containerName, verb, label string) error {
	if err := b.provisioner.ensure(ctx, containerName); err != nil {
		return err
	}
	return b.runtime.start(ctx, containerName, verb, label)
}

// StopAgentBrowser tears down the browser, VNC bridge, and virtual display.
func (c *Client) StopAgentBrowser(ctx context.Context, containerName string) error {
	return c.browser.runtime.stop(ctx, containerName)
}

// StopAgentBrowserView tears down only the noVNC/VNC layer.
func (c *Client) StopAgentBrowserView(ctx context.Context, containerName string) error {
	return c.browser.runtime.stopView(ctx, containerName)
}

// AgentBrowserRunning reports whether the core is currently ready.
func (c *Client) AgentBrowserRunning(ctx context.Context, containerName string) (bool, error) {
	return c.browser.runtime.running(ctx, containerName)
}

// AgentBrowserStatus returns the split core/view state reported by gui-up.sh.
func (c *Client) AgentBrowserStatus(ctx context.Context, containerName string) (serviceproject.AgentBrowserInfo, error) {
	return c.browser.runtime.status(ctx, containerName)
}
