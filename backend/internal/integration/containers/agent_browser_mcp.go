package containers

// Agent Browser MCP provisioning: makes the @playwright/mcp browser tools
// available to in-container agents, attached over CDP to the live Chrome (the
// SAME session the user logs into). This is the tool layer behind the
// `browser` skill — the agent calls browser_navigate / browser_snapshot /
// browser_click / browser_type etc. instead of hand-writing Playwright recipes.
//
// Only wired when the browser skill is selected (see the providers), so the
// tool surface and the per-prompt MCP process don't burden ordinary prompts.

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"
)

const (
	browserMCPInstallTimeout = 5 * time.Minute
)

// EnsureAgentBrowserMCP installs @playwright/mcp (idempotently) and pushes the
// profile-owned MCP templates. Cheap once installed: the npm-presence check
// short-circuits, and templates are only re-pushed when their content changes.
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

	for _, profile := range c.AgentProfiles() {
		for _, template := range profile.BrowserMCPTemplates {
			directory := template.Directory
			if directory == "" {
				directory = path.Dir(template.Path)
			}
			directoryMode := template.DirectoryMode
			if directoryMode == "" {
				directoryMode = "755"
			}
			dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
			out, err := c.lxc.Run(dctx, "exec", containerName, "--",
				"install", "-d", "-m", directoryMode, directory)
			cancelD()
			if err != nil {
				return fmt.Errorf("mkdir %s: %w; output: %s", directory, err, out)
			}
			mode := template.Mode
			if mode == "" {
				mode = "644"
			}
			if err := c.templates.push(ctx, containerName, template.Content,
				template.HashPath, mode, template.Path); err != nil {
				return err
			}
		}
	}
	return nil
}
