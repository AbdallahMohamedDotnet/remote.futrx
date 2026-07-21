package containers

// Container provisioning: ships a project's CLAUDE.md / AGENTS.md template
// into the container. Claude-specific, but kept in its own file because of
// the //go:embed directive.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

var agentInstructionsTemplate = provisioning.InstructionsTemplate()

const (
	containerClaudeMD         = "/root/.claude/CLAUDE.md"
	containerCodexAGENTS      = "/root/.codex/AGENTS.md"
	containerAgentInstrMDHash = "/root/.claude/.agents-md.sha256"
)

// EnsureAgentInstructions pushes the agent system-instructions template into
// both the Claude (/root/.claude/CLAUDE.md) and Codex (/root/.codex/AGENTS.md)
// homes, gated by a single sha256 marker. Idempotent.
func (c *Client) EnsureAgentInstructions(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}

	dctx, cancelD := context.WithTimeout(ctx, 30*time.Second)
	defer cancelD()
	if out, err := c.lxc.Run(dctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", containerClaudeDir); err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerClaudeDir, err, out)
	}
	if out, err := c.lxc.Run(dctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", "/root/.codex"); err != nil {
		return fmt.Errorf("mkdir /root/.codex: %w; output: %s", err, out)
	}

	return c.pushTemplatedFile(ctx, containerName, agentInstructionsTemplate,
		containerAgentInstrMDHash, "644", containerClaudeMD, containerCodexAGENTS)
}
