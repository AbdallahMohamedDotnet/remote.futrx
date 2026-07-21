package containers

// Agent Browser MCP provisioning: makes the @playwright/mcp browser tools
// available to the in-container agent, attached over CDP to the live Chrome (the
// SAME session the user logs into). This is the tool layer behind the
// `browser` skill — the agent calls browser_navigate / browser_snapshot /
// browser_click / browser_type etc. instead of hand-writing Playwright recipes.
//
// Only wired when the browser skill is selected (see the providers), so the
// tool surface and the per-prompt MCP process don't burden ordinary prompts.

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"
)

//go:embed templates/mcp-claude.json
var mcpClaudeConfig []byte

const (
	// ContainerMCPClaudeConfig is the --mcp-config file claude is pointed at
	// when the browser skill is active. It registers the @playwright/mcp
	// server attached to the loopback CDP endpoint.
	ContainerMCPClaudeConfig = containerGUIDir + "/mcp-claude.json"
	containerMCPConfigHash   = containerGUIDir + "/.mcp-claude.sha256"

	browserMCPInstallTimeout = 5 * time.Minute
)

// EnsureAgentBrowserMCP installs @playwright/mcp (idempotently) and pushes the
// Claude MCP config. Codex uses equivalent inline config flags, but shares this
// package install step. Cheap once installed: the npm-presence check short-
// circuits, and the config is only re-pushed when its embedded content changes.
func (c *Client) EnsureAgentBrowserMCP(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}

	cctx, cancelC := context.WithTimeout(ctx, queryTimeout)
	_, missing := c.lxc.Run(cctx, "exec", containerName, "--", "sh", "-c", "npm ls -g @playwright/mcp >/dev/null 2>&1")
	cancelC()
	if missing != nil {
		ictx, cancelI := context.WithTimeout(ctx, browserMCPInstallTimeout)
		out, err := c.lxc.Run(ictx, "exec", containerName, "--", "sh", "-c", "npm install -g @playwright/mcp 2>&1 | tail -3")
		cancelI()
		if err != nil {
			return fmt.Errorf("install @playwright/mcp: %w; output: %s", err, truncateOut(out, 1000))
		}
	}

	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	out, err := c.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancelD()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	return c.pushTemplatedFile(ctx, containerName, mcpClaudeConfig, containerMCPConfigHash, "644", ContainerMCPClaudeConfig)
}
