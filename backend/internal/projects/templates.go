package projects

// Embedded per-project templates pushed into each container so the in-container
// agent knows the shape of its sandbox. Re-pushed by EnsureClaudeMD whenever
// the embedded content's hash differs from the marker stored in the container,
// so editing templates/CLAUDE.md + redeploying automatically updates every
// container on its next prompt.

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed templates/CLAUDE.md
var claudeMDTemplate []byte

const (
	// Path inside the container where the template lands. /root/.claude/CLAUDE.md
	// is the canonical user-level memory location claude reads on every session
	// (alongside walking up from $CWD for project-level CLAUDE.md files).
	// Earlier we used /root/CLAUDE.md — HOME directly — which claude does NOT
	// auto-load.
	containerClaudeMD = "/root/.claude/CLAUDE.md"
	// Marker file holding the SHA256 of the last-pushed template.
	containerClaudeMDHash = "/root/.claude/.claude-md.sha256"
)

func claudeMDHash() string {
	sum := sha256.Sum256(claudeMDTemplate)
	return hex.EncodeToString(sum[:])
}

// EnsureClaudeMD pushes /root/CLAUDE.md into the container if it's missing
// or stale. Cheap (one lxc exec + a hash compare) when up-to-date.
func (m *Manager) EnsureClaudeMD(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	want := claudeMDHash()

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	defer cancelQ()
	got, err := lxcRun(qctx, "exec", containerName, "--", "cat", containerClaudeMDHash)
	if err == nil && strings.TrimSpace(got) == want {
		return nil
	}

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	// Stage the bytes in a host temp file so `lxc file push` has a source.
	tmp, err := os.CreateTemp("", "claude-md-*.md")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(claudeMDTemplate); err != nil {
		tmp.Close()
		return fmt.Errorf("write template: %w", err)
	}
	tmp.Close()

	if out, err := lxcRun(pctx, "file", "push", "--mode=644",
		tmp.Name(), containerName+containerClaudeMD); err != nil {
		return fmt.Errorf("push CLAUDE.md: %w; output: %s", err, out)
	}

	// Update the marker. /root/.claude already exists post-EnsureClaudeAuth,
	// but tolerate the race where this runs first by mkdir -p'ing it.
	if out, err := lxcRun(pctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", "/root/.claude"); err != nil {
		return fmt.Errorf("mkdir /root/.claude: %w; output: %s", err, out)
	}
	hashCmd := exec.CommandContext(pctx, "lxc", "exec", containerName, "--",
		"tee", containerClaudeMDHash)
	hashCmd.Stdin = strings.NewReader(want)
	if out, err := hashCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write hash marker: %w; output: %s", err, out)
	}
	return nil
}
