// Package browser provisions and controls the project container's shared
// headed-browser feature and its agent tooling.
package browser

// Agent Browser provisioning: brings up a real headed Google Chrome inside
// the project container, rendered on a virtual display (Xvfb) and exposed two
// ways onto the SAME session - a noVNC web view the user logs in through, and
// a loopback CDP port the agent drives. The launcher script
// (assets/gui-up.sh) is workspace-resident so it survives container deletes; the host
// re-pushes it whenever the embedded template changes (sha256 marker, same
// pattern as browser.mjs / AGENTS.md).

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// VNCPort is the in-container port the noVNC/websockify front
// listens on. It is the only externally-reachable port of the GUI stack and is
// surfaced to the user through the existing dev-URL proxy.
const VNCPort = 6080

// Service owns browser installation, workspace assets, MCP integration,
// container configuration, and the split core/view runtime.
type Service struct {
	runner    command.Runner
	publisher *assets.Publisher
	browser   agentBrowser
	mcp       agentBrowserMCPProvisioner
	config    agentBrowserConfigurator
}

// NewService returns a browser service backed by shared container dependencies.
func NewService(runner command.Runner, profileSource serviceprofiles.Source, publisher *assets.Publisher) *Service {
	return &Service{
		runner:    runner,
		publisher: publisher,
		browser: agentBrowser{
			provisioner: agentBrowserProvisioner{runner: runner, publisher: publisher},
			runtime:     agentBrowserRuntime{runner: runner},
		},
		mcp: agentBrowserMCPProvisioner{
			runner:    runner,
			profiles:  profileSource,
			publisher: publisher,
		},
		config: agentBrowserConfigurator{runner: runner},
	}
}

// agentBrowser owns installation, workspace templates, and the split core/view
// runtime lifecycle.
type agentBrowser struct {
	provisioner agentBrowserProvisioner
	runtime     agentBrowserRuntime
}

// AgentBrowserPort returns the in-container noVNC port the stack listens on.
func (s *Service) Port() int { return VNCPort }

// EnsureAgentBrowser starts the full stack: browser core plus noVNC view.
func (s *Service) Ensure(ctx context.Context, containerName string) error {
	return s.browser.start(ctx, containerName, "start", "start agent browser")
}

// EnsureAgentBrowserCore starts only Xvfb, openbox, headed Chromium, and CDP.
func (s *Service) EnsureCore(ctx context.Context, containerName string) error {
	return s.browser.start(ctx, containerName, "start-core", "start agent browser core")
}

// EnsureAgentBrowserView starts the noVNC/VNC layer on top of the same core.
func (s *Service) EnsureView(ctx context.Context, containerName string) error {
	return s.browser.start(ctx, containerName, "start-view", "start agent browser view")
}

func (b *agentBrowser) start(ctx context.Context, containerName, verb, label string) error {
	if err := b.provisioner.ensure(ctx, containerName); err != nil {
		return err
	}
	return b.runtime.start(ctx, containerName, verb, label)
}

// StopAgentBrowser tears down the browser, VNC bridge, and virtual display.
func (s *Service) Stop(ctx context.Context, containerName string) error {
	return s.browser.runtime.stop(ctx, containerName)
}

// StopAgentBrowserView tears down only the noVNC/VNC layer.
func (s *Service) StopView(ctx context.Context, containerName string) error {
	return s.browser.runtime.stopView(ctx, containerName)
}

// AgentBrowserRunning reports whether the core is currently ready.
func (s *Service) Running(ctx context.Context, containerName string) (bool, error) {
	return s.browser.runtime.running(ctx, containerName)
}

// AgentBrowserStatus returns the split core/view state reported by gui-up.sh.
func (s *Service) Status(ctx context.Context, containerName string) (serviceproject.AgentBrowserInfo, error) {
	return s.browser.runtime.status(ctx, containerName)
}
