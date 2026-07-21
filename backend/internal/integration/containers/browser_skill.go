package containers

// Browser-skill provisioning: ships the `browser` SKILL.md into the workspace
// so it shows up in the skill picker and is available to the agent. The skill
// holds the browser playbook (how to use the browser_* tools, the login
// handoff, the write-approval policy); selecting it is what wires the MCP
// tools (see the providers). Provisioned at container launch like the browser
// script, so every project has it without bloating AGENTS.md.

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
)

//go:embed templates/skills/browser/SKILL.md
var browserSkillTemplate []byte

const (
	containerBrowserSkillDir  = "/workspace/.agents/skills/browser"
	containerBrowserSkillMD   = containerBrowserSkillDir + "/SKILL.md"
	containerBrowserSkillHash = containerBrowserSkillDir + "/.skill.sha256"
)

// EnsureBrowserSkill provisions the `browser` skill into the workspace skills
// directory. Idempotent: re-pushed only when the embedded SKILL.md changes.
func (c *Client) EnsureBrowserSkill(ctx context.Context, containerName string) error {
	return c.workspace.ensureBrowserSkill(ctx, containerName)
}

func (p *workspaceProvisioner) ensureBrowserSkill(ctx context.Context, containerName string) error {
	if !p.lxc.Available() {
		return errors.New("lxc not available")
	}
	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	out, err := p.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerBrowserSkillDir)
	cancelD()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerBrowserSkillDir, err, out)
	}
	return p.templates.push(ctx, containerName, browserSkillTemplate, containerBrowserSkillHash, "644", containerBrowserSkillMD)
}
