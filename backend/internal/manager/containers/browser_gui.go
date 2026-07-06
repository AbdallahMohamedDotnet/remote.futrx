package containers

// Agent Browser provisioning: brings up a real headed Google Chrome inside
// the project container, rendered on a virtual display (Xvfb) and exposed two
// ways onto the SAME session — a noVNC web view the user logs in through, and
// a loopback CDP port the agent drives. The launcher script (templates/
// gui-up.sh) is workspace-resident so it survives container deletes; the host
// re-pushes it whenever the embedded template changes (sha256 marker, same
// pattern as browser.mjs / AGENTS.md).
//
// The Chrome profile lives under /workspace, so a login the user performs
// through the noVNC view persists across container restarts. Egress is the
// container's own (datacenter) network — there is no traffic routing in this
// version. See templates/gui-up.sh for the process tree it manages.

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

//go:embed templates/gui-up.sh
var guiUpScript []byte

//go:embed templates/human-input.sh
var humanInputScript []byte

const (
	// BrowserGUIVNCPort is the in-container port the noVNC/websockify front
	// listens on. It is the only externally-reachable port of the GUI stack
	// and is surfaced to the user through the existing dev-URL proxy at
	// <slug>--6080.dev.<host>, behind the platform's Google auth gate.
	BrowserGUIVNCPort = 6080

	// browserGUICDPPort is Chrome's remote-debugging port. It binds to
	// loopback inside the container so only the in-container agent (and the
	// readiness check) can attach; it is never proxied out.
	browserGUICDPPort = 9222

	containerGUIDir           = "/workspace/.browser-gui"
	containerGUIScript        = containerGUIDir + "/gui-up.sh"
	containerGUIScriptHash    = containerGUIDir + "/.gui-up.sha256"
	containerHumanInputScript = containerGUIDir + "/human-input.sh"
	containerHumanInputHash   = containerGUIDir + "/.human-input.sha256"

	browserGUIReadyTimeout   = 60 * time.Second
	browserGUIInstallTimeout = 8 * time.Minute

	agentBrowserStatusStopped  = "stopped"
	agentBrowserStatusCoreOnly = "core-ready"
	agentBrowserStatusReady    = "ready"
)

// BrowserGUIPort returns the in-container noVNC port the GUI stack listens
// on — the single source of truth for callers that build the dev-URL.
func (m *Manager) BrowserGUIPort() int { return BrowserGUIVNCPort }

// EnsureBrowserGUI provisions and starts the Agent Browser stack inside the
// container (Xvfb -> openbox -> Google Chrome -> x11vnc -> websockify/noVNC),
// returning once it is reachable. Idempotent: an already-running stack is a
// fast no-op, and the launcher script is only re-pushed when its embedded
// content changes.
//
// If the container pre-dates the GUI stack being baked into the base image,
// the dependencies are installed on demand (the same recipe BuildBaseImage
// layers on), so the feature works without a full image rebuild.
func (m *Manager) EnsureBrowserGUI(ctx context.Context, containerName string) error {
	return m.ensureBrowserGUI(ctx, containerName, "start", "start browser GUI")
}

// EnsureBrowserGUICore starts only the agent-facing browser core: virtual
// display, window manager, headed Chromium, and loopback CDP. It deliberately
// leaves the VNC/noVNC view off so an agent can use the browser without paying
// for framebuffer streaming.
func (m *Manager) EnsureBrowserGUICore(ctx context.Context, containerName string) error {
	return m.ensureBrowserGUI(ctx, containerName, "start-core", "start browser GUI core")
}

// EnsureBrowserGUIView starts the human-facing noVNC layer on top of the same
// browser core. The launcher starts core first if needed.
func (m *Manager) EnsureBrowserGUIView(ctx context.Context, containerName string) error {
	return m.ensureBrowserGUI(ctx, containerName, "start-view", "start browser GUI view")
}

func (m *Manager) ensureBrowserGUI(ctx context.Context, containerName, verb, label string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}

	cctx, cancelC := context.WithTimeout(ctx, queryTimeout)
	_, stackErr := m.lxc.Run(cctx, "exec", containerName, "--", "sh", "-c", "command -v Xvfb >/dev/null 2>&1 && ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome >/dev/null 2>&1")
	cancelC()
	if stackErr != nil {
		ictx, cancelI := context.WithTimeout(ctx, browserGUIInstallTimeout)
		out, err := m.lxc.Run(ictx, "exec", containerName, "--", "bash", "-c", BrowserGUIInstallScript)
		cancelI()
		if err != nil {
			return fmt.Errorf("install browser GUI stack: %w; output: %s", err, truncateOut(out, 2000))
		}
	}

	if err := m.pushGUITemplates(ctx, containerName); err != nil {
		return err
	}

	sctx, cancelS := context.WithTimeout(ctx, browserGUIReadyTimeout)
	defer cancelS()
	if out, err := m.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, verb); err != nil {
		return fmt.Errorf("%s: %w; output: %s", label, err, truncateOut(out, 1000))
	}
	return nil
}

// pushGUITemplates ensures the launcher dir exists, then (re)pushes the
// scripts when their embedded content has changed.
func (m *Manager) pushGUITemplates(ctx context.Context, containerName string) error {
	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	out, err := m.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancelD()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	if err := m.pushTemplatedFile(ctx, containerName, guiUpScript, containerGUIScriptHash, "755", containerGUIScript); err != nil {
		return err
	}
	return m.pushTemplatedFile(ctx, containerName, humanInputScript, containerHumanInputHash, "755", containerHumanInputScript)
}

// StopBrowserGUI tears down the GUI stack (browser, VNC, display) but leaves
// the persistent profile on disk so logins survive. Best-effort.
func (m *Manager) StopBrowserGUI(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	sctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := m.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, "stop"); err != nil {
		return fmt.Errorf("stop browser GUI: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}

// StopBrowserGUIView tears down only the noVNC/VNC layer. Chrome and CDP stay
// alive so an in-container agent can continue using the same logged-in session.
func (m *Manager) StopBrowserGUIView(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	sctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := m.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, "stop-view"); err != nil {
		return fmt.Errorf("stop browser GUI view: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}

// BrowserGUIRunning reports whether the GUI stack is currently up and
// reachable in the container. A missing/unprovisioned script reports false
// rather than erroring.
func (m *Manager) BrowserGUIRunning(ctx context.Context, containerName string) (bool, error) {
	info, err := m.BrowserGUIStatus(ctx, containerName)
	if err != nil {
		return false, err
	}
	return info.Core == "ready", nil
}

// BrowserGUIStatus returns the split core/view state reported by gui-up.sh.
// Missing or not-yet-provisioned scripts report a stopped browser rather than
// bubbling an error to status pollers.
func (m *Manager) BrowserGUIStatus(ctx context.Context, containerName string) (serviceproject.AgentBrowserInfo, error) {
	if !m.Available() {
		return serviceproject.AgentBrowserInfo{}, errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(qctx, "exec", containerName, "--", "sh", containerGUIScript, "status")
	if err != nil {
		return serviceproject.AgentBrowserInfo{
			Status: agentBrowserStatusStopped,
			Core:   "off",
			View:   "off",
		}, nil
	}
	return parseBrowserGUIStatus(out), nil
}

func parseBrowserGUIStatus(out string) serviceproject.AgentBrowserInfo {
	info := serviceproject.AgentBrowserInfo{
		Status: agentBrowserStatusStopped,
		Core:   "off",
		View:   "off",
	}
	for _, field := range strings.Fields(out) {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "core":
			info.Core = value
		case "view":
			info.View = value
		case "clients":
			if n, err := strconv.Atoi(value); err == nil {
				info.ViewerCount = n
			}
		case "uptime_sec":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				info.UptimeSec = n
			}
		}
	}
	switch {
	case info.Core == "ready" && info.View == "ready":
		info.Status = agentBrowserStatusReady
	case info.Core == "ready":
		info.Status = agentBrowserStatusCoreOnly
	default:
		info.Status = agentBrowserStatusStopped
	}
	return info
}

// EnsureBrowserGUILimits applies the container config a headed browser needs:
// security.nesting (so Chrome's namespaces work even if --no-sandbox is later
// dropped). Older versions also applied container-wide CPU/memory caps here;
// this now removes those keys so the browser budget is scoped to Chrome's own
// process tree by gui-up.sh instead of throttling builds and dev servers.
func (m *Manager) EnsureBrowserGUILimits(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	cur, _ := m.lxc.Run(lctx, "config", "get", containerName, "security.nesting")
	if strings.TrimSpace(cur) != "true" {
		out, err := m.lxc.Run(lctx, "config", "set", containerName, "security.nesting", "true")
		if err != nil {
			cancel()
			return fmt.Errorf("set security.nesting: %w; output: %s", err, out)
		}
	}
	cancel()

	for _, key := range []string{"limits.cpu", "limits.memory"} {
		qctx, qcancel := context.WithTimeout(ctx, queryTimeout)
		cur, _ := m.lxc.Run(qctx, "config", "get", containerName, key)
		qcancel()
		if strings.TrimSpace(cur) == "" {
			continue
		}
		uctx, ucancel := context.WithTimeout(ctx, queryTimeout)
		out, err := m.lxc.Run(uctx, "config", "unset", containerName, key)
		ucancel()
		if err != nil {
			return fmt.Errorf("unset %s: %w; output: %s", key, err, out)
		}
	}
	return nil
}
