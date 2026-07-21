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
	// AgentBrowserVNCPort is the in-container port the noVNC/websockify front
	// listens on. It is the only externally-reachable port of the GUI stack
	// and is surfaced to the user through the existing dev-URL proxy.
	AgentBrowserVNCPort = 6080

	// agentBrowserCDPPort is Chrome's remote-debugging port. It binds to
	// loopback inside the container so only the in-container agent can attach.
	agentBrowserCDPPort = 9222

	containerGUIDir           = "/workspace/.browser-gui"
	containerGUIScript        = containerGUIDir + "/gui-up.sh"
	containerGUIScriptHash    = containerGUIDir + "/.gui-up.sha256"
	containerHumanInputScript = containerGUIDir + "/human-input.sh"
	containerHumanInputHash   = containerGUIDir + "/.human-input.sha256"

	agentBrowserReadyTimeout   = 60 * time.Second
	agentBrowserInstallTimeout = 8 * time.Minute
)

// AgentBrowserPort returns the in-container noVNC port the stack listens on.
func (c *Client) AgentBrowserPort() int { return AgentBrowserVNCPort }

// EnsureAgentBrowser starts the full stack: browser core plus noVNC view.
func (c *Client) EnsureAgentBrowser(ctx context.Context, containerName string) error {
	return c.ensureAgentBrowser(ctx, containerName, "start", "start agent browser")
}

// EnsureAgentBrowserCore starts only Xvfb, openbox, headed Chromium, and CDP.
func (c *Client) EnsureAgentBrowserCore(ctx context.Context, containerName string) error {
	return c.ensureAgentBrowser(ctx, containerName, "start-core", "start agent browser core")
}

// EnsureAgentBrowserView starts the noVNC/VNC layer on top of the same core.
func (c *Client) EnsureAgentBrowserView(ctx context.Context, containerName string) error {
	return c.ensureAgentBrowser(ctx, containerName, "start-view", "start agent browser view")
}

func (c *Client) ensureAgentBrowser(ctx context.Context, containerName, verb, label string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}

	cctx, cancelC := context.WithTimeout(ctx, queryTimeout)
	_, stackErr := c.lxc.Run(cctx, "exec", containerName, "--", "sh", "-c", "command -v Xvfb >/dev/null 2>&1 && ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome >/dev/null 2>&1")
	cancelC()
	if stackErr != nil {
		ictx, cancelI := context.WithTimeout(ctx, agentBrowserInstallTimeout)
		out, err := c.lxc.Run(ictx, "exec", containerName, "--", "bash", "-c", AgentBrowserInstallScript)
		cancelI()
		if err != nil {
			return fmt.Errorf("install agent browser stack: %w; output: %s", err, truncateOut(out, 2000))
		}
	}

	if err := c.pushAgentBrowserTemplates(ctx, containerName); err != nil {
		return err
	}

	sctx, cancelS := context.WithTimeout(ctx, agentBrowserReadyTimeout)
	defer cancelS()
	if out, err := c.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, verb); err != nil {
		return fmt.Errorf("%s: %w; output: %s", label, err, truncateOut(out, 1000))
	}
	return nil
}

func (c *Client) pushAgentBrowserTemplates(ctx context.Context, containerName string) error {
	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	out, err := c.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancelD()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	if err := c.pushTemplatedFile(ctx, containerName, guiUpScript, containerGUIScriptHash, "755", containerGUIScript); err != nil {
		return err
	}
	return c.pushTemplatedFile(ctx, containerName, humanInputScript, containerHumanInputHash, "755", containerHumanInputScript)
}

// StopAgentBrowser tears down the browser, VNC bridge, and virtual display.
func (c *Client) StopAgentBrowser(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	sctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := c.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, "stop"); err != nil {
		return fmt.Errorf("stop agent browser: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}

// StopAgentBrowserView tears down only the noVNC/VNC layer.
func (c *Client) StopAgentBrowserView(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	sctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := c.lxc.Run(sctx, "exec", containerName, "--", "sh", containerGUIScript, "stop-view"); err != nil {
		return fmt.Errorf("stop agent browser view: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}

// AgentBrowserRunning reports whether the core is currently ready.
func (c *Client) AgentBrowserRunning(ctx context.Context, containerName string) (bool, error) {
	info, err := c.AgentBrowserStatus(ctx, containerName)
	if err != nil {
		return false, err
	}
	return info.Core == "ready", nil
}

// AgentBrowserStatus returns the split core/view state reported by gui-up.sh.
func (c *Client) AgentBrowserStatus(ctx context.Context, containerName string) (serviceproject.AgentBrowserInfo, error) {
	if !c.Available() {
		return serviceproject.AgentBrowserInfo{}, errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.lxc.Run(qctx, "exec", containerName, "--", "sh", containerGUIScript, "status")
	if err != nil {
		return serviceproject.AgentBrowserInfo{
			Status: serviceproject.AgentBrowserStatusStopped,
			Core:   "off",
			View:   "off",
		}, nil
	}
	return parseAgentBrowserStatus(out), nil
}

func parseAgentBrowserStatus(out string) serviceproject.AgentBrowserInfo {
	info := serviceproject.AgentBrowserInfo{
		Status: serviceproject.AgentBrowserStatusStopped,
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
		info.Status = serviceproject.AgentBrowserStatusReady
	case info.Core == "ready":
		info.Status = serviceproject.AgentBrowserStatusCoreReady
	default:
		info.Status = serviceproject.AgentBrowserStatusStopped
	}
	return info
}

// EnsureAgentBrowserLimits applies only container config that Chrome needs and
// removes older browser-induced container-wide CPU/memory limits.
func (c *Client) EnsureAgentBrowserLimits(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	cur, _ := c.lxc.Run(lctx, "config", "get", containerName, "security.nesting")
	if strings.TrimSpace(cur) != "true" {
		out, err := c.lxc.Run(lctx, "config", "set", containerName, "security.nesting", "true")
		if err != nil {
			cancel()
			return fmt.Errorf("set security.nesting: %w; output: %s", err, out)
		}
	}
	cancel()

	for _, key := range []string{"limits.cpu", "limits.memory"} {
		qctx, qcancel := context.WithTimeout(ctx, queryTimeout)
		cur, _ := c.lxc.Run(qctx, "config", "get", containerName, key)
		qcancel()
		if strings.TrimSpace(cur) == "" {
			continue
		}
		uctx, ucancel := context.WithTimeout(ctx, queryTimeout)
		out, err := c.lxc.Run(uctx, "config", "unset", containerName, key)
		ucancel()
		if err != nil {
			return fmt.Errorf("unset %s: %w; output: %s", key, err, out)
		}
	}
	return nil
}
