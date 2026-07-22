package browser

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

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

//go:embed assets/skills/browser/SKILL.md
var browserSkillTemplate []byte

const (
	containerBrowserSkillDir  = "/workspace/.agents/skills/browser"
	containerBrowserSkillMD   = containerBrowserSkillDir + "/SKILL.md"
	containerBrowserSkillHash = containerBrowserSkillDir + "/.skill.sha256"
)

// EnsureBrowserSkill provisions the `browser` skill into the workspace skills
// directory. Idempotent: re-pushed only when the embedded SKILL.md changes.
func (s *Adapter) EnsureSkill(ctx context.Context, containerName string) error {
	if !s.runner.Available() {
		return errors.New("lxc not available")
	}
	out, err := command.RunWithTimeout(ctx, s.runner, queryTimeout, "exec", containerName, "--", "install", "-d", "-m", "755", containerBrowserSkillDir)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerBrowserSkillDir, err, out)
	}
	return s.publisher.Push(ctx, containerName, browserSkillTemplate, containerBrowserSkillHash, "644", containerBrowserSkillMD)
}
