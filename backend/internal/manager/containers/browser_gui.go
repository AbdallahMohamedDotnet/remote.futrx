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
	"strings"
	"time"
)

//go:embed templates/gui-up.sh
var guiUpScript []byte

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

	containerGUIDir        = "/workspace/.browser-gui"
	containerGUIScript     = containerGUIDir + "/gui-up.sh"
	containerGUIScriptHash = containerGUIDir + "/.gui-up.sha256"

	// browserGUICPULimit / browserGUIMemLimit cap a software-rendered browser
	// so it cannot starve the shared host. Applied by EnsureBrowserGUILimits.
	browserGUICPULimit = "2"
	browserGUIMemLimit = "3GB"

	browserGUIReadyTimeout   = 60 * time.Second
	browserGUIInstallTimeout = 8 * time.Minute
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

	if err := m.pushGUIScript(ctx, containerName); err != nil {
		return err
	}

	sctx, cancelS := context.WithTimeout(ctx, browserGUIReadyTimeout)
	defer cancelS()
	if out, err := m.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, "start"); err != nil {
		return fmt.Errorf("start browser GUI: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}

// pushGUIScript ensures the launcher dir exists, then (re)pushes gui-up.sh
// when its embedded content has changed.
func (m *Manager) pushGUIScript(ctx context.Context, containerName string) error {
	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	out, err := m.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancelD()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	return m.pushTemplatedFile(ctx, containerName, guiUpScript, containerGUIScriptHash, "755", containerGUIScript)
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

// BrowserGUIRunning reports whether the GUI stack is currently up and
// reachable in the container. A missing/unprovisioned script reports false
// rather than erroring.
func (m *Manager) BrowserGUIRunning(ctx context.Context, containerName string) (bool, error) {
	if !m.Available() {
		return false, errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(qctx, "exec", containerName, "--", "sh", containerGUIScript, "status")
	if err != nil {
		return false, nil
	}
	return strings.Contains(out, "ready") && !strings.Contains(out, "not ready"), nil
}

// EnsureBrowserGUILimits applies the container config a headed browser needs:
// security.nesting (so Chrome's namespaces work even if --no-sandbox is later
// dropped) plus CPU and memory caps so a software-rendered browser cannot
// starve the shared host. Idempotent; safe to call on every launch as a
// migration for older containers.
func (m *Manager) EnsureBrowserGUILimits(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	settings := [][2]string{
		{"security.nesting", "true"},
		{"limits.cpu", browserGUICPULimit},
		{"limits.memory", browserGUIMemLimit},
	}
	for _, kv := range settings {
		lctx, cancel := context.WithTimeout(ctx, queryTimeout)
		cur, _ := m.lxc.Run(lctx, "config", "get", containerName, kv[0])
		if strings.TrimSpace(cur) == kv[1] {
			cancel()
			continue
		}
		out, err := m.lxc.Run(lctx, "config", "set", containerName, kv[0], kv[1])
		cancel()
		if err != nil {
			return fmt.Errorf("set %s: %w; output: %s", kv[0], err, out)
		}
	}
	return nil
}
