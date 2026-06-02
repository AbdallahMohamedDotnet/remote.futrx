package containers

// Container provisioning: ships a project's CLAUDE.md template into the
// container. Claude-specific, but kept in its own file because of the
// //go:embed directive.

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

//go:embed templates/AGENTS.md
var agentInstructionsTemplate []byte

const (
	containerClaudeMD          = "/root/.claude/CLAUDE.md"
	containerCodexAGENTS       = "/root/.codex/AGENTS.md"
	containerAgentInstrMDHash  = "/root/.claude/.agents-md.sha256"
)

func agentInstructionsHash() string {
	sum := sha256.Sum256(agentInstructionsTemplate)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) EnsureAgentInstructions(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	want := agentInstructionsHash()

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	defer cancelQ()
	got, err := m.lxc.Run(qctx, "exec", containerName, "--", "cat", containerAgentInstrMDHash)
	if err == nil && strings.TrimSpace(got) == want {
		return nil
	}

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	tmp, err := os.CreateTemp("", "claude-md-*.md")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(agentInstructionsTemplate); err != nil {
		tmp.Close()
		return fmt.Errorf("write template: %w", err)
	}
	tmp.Close()

	if out, err := m.lxc.Run(pctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", containerClaudeDir); err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerClaudeDir, err, out)
	}
	if out, err := m.lxc.Run(pctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", "/root/.codex"); err != nil {
		return fmt.Errorf("mkdir /root/.codex: %w; output: %s", err, out)
	}
	if out, err := m.lxc.Run(pctx, "file", "push", "--mode=644",
		tmp.Name(), containerName+containerClaudeMD); err != nil {
		return fmt.Errorf("push CLAUDE.md: %w; output: %s", err, out)
	}
	if out, err := m.lxc.Run(pctx, "file", "push", "--mode=644",
		tmp.Name(), containerName+containerCodexAGENTS); err != nil {
		return fmt.Errorf("push AGENTS.md: %w; output: %s", err, out)
	}
	if out, err := m.lxc.RunStdin(pctx, strings.NewReader(want), "exec", containerName, "--",
		"tee", containerAgentInstrMDHash); err != nil {
		return fmt.Errorf("write hash marker: %w; output: %s", err, out)
	}
	return nil
}
