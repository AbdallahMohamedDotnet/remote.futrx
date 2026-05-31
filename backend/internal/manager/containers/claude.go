package containers

// Claude-specific provider glue: installing the CLI inside a container and
// the AuthBundle that describes Claude's credentials. Anything generic
// (lifecycle, push/pull pipeline) lives in the other files; this file is
// where Claude knowledge is concentrated.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// Host paths — the canonical Claude credentials live in /root/.claude*.
	// The host runs `claude auth login` once via the claudelogin manager,
	// then we ship the resulting files into every container that needs them.
	hostClaudeJSON  = "/root/.claude.json"
	hostClaudeDir   = "/root/.claude"
	hostClaudeCreds = "/root/.claude/.credentials.json"

	// Container paths — same layout, just under the container's root home.
	containerClaudeJSON  = "/root/.claude.json"
	containerClaudeDir   = "/root/.claude"
	containerClaudeCreds = "/root/.claude/.credentials.json"
)

// ClaudeAuthBundle is the AuthBundle definition for Anthropic's Claude CLI.
// Register it on a Manager (typically in cmd/remote/main.go) to have Claude
// credentials auto-seeded into every freshly launched container.
func ClaudeAuthBundle() AuthBundle {
	return AuthBundle{
		Name:         "claude",
		HostDir:      hostClaudeDir,
		ContainerDir: containerClaudeDir,
		// The original implementation attached the credentials as a `disk`
		// device named "claude-auth". Remove it on every push so older
		// containers migrate cleanly.
		LegacyDevices: []string{"claude-auth"},
		Files: []AuthFile{
			{
				HostPath:      hostClaudeJSON,
				ContainerPath: containerClaudeJSON,
				Mode:          "600",
				PushRequired:  true, // gate: if missing, host isn't logged in
				PullRequired:  true,
			},
			{
				HostPath:      hostClaudeCreds,
				ContainerPath: containerClaudeCreds,
				Mode:          "600",
				PushRequired:  false, // first-launch path: may not exist yet
				PullRequired:  true,  // after a run, Claude must have written it
			},
		},
	}
}

// EnsureClaudeAuth seeds the Claude credential bundle into the container.
// Kept as a thin wrapper so existing callers (claude provider, prompt
// service) keep working; new code can call EnsureAuthBundle directly with
// any bundle.
func (m *Manager) EnsureClaudeAuth(ctx context.Context, containerName string) error {
	return m.EnsureAuthBundle(ctx, containerName, ClaudeAuthBundle())
}

// SyncClaudeAuthFromContainer pulls Claude credentials back to the host
// after a run, so any OAuth refresh that happened inside the container
// survives.
func (m *Manager) SyncClaudeAuthFromContainer(ctx context.Context, containerName string) error {
	return m.SyncAuthBundleFromContainer(ctx, containerName, ClaudeAuthBundle())
}

// EnsureClaude installs the Anthropic Claude CLI inside the container if it
// isn't already present. Safe to call on every prompt — no-ops once
// installed; coalesces concurrent installs by polling when one is already
// running.
func (m *Manager) EnsureClaude(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	if m.claudeInstalled(ctx, containerName) {
		return nil
	}
	if m.claudeInstallRunning(ctx, containerName) {
		waitCtx, cancelW := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelW()
		if err := m.waitForClaude(waitCtx, containerName); err == nil {
			return nil
		}
	}

	installCtx, cancelI := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelI()
	script := `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates gnupg
curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null 2>&1
apt-get install -y -qq nodejs
npm install -g @anthropic-ai/claude-code --silent 2>&1 | tail -3
which claude && claude --version`
	out, err := m.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", script)
	if err != nil {
		waitCtx, cancelW := context.WithTimeout(ctx, 90*time.Second)
		defer cancelW()
		if waitErr := m.waitForClaude(waitCtx, containerName); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install claude in %s: %w; output: %s",
			containerName, err, truncateOut(out, 1000))
	}
	return nil
}

func (m *Manager) claudeInstalled(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := m.lxc.Run(quickCtx, "exec", containerName, "--", "which", "claude")
	return err == nil
}

func (m *Manager) claudeInstallRunning(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*@anthropic-ai/claude-code")
	return err == nil && strings.TrimSpace(out) != ""
}

func (m *Manager) waitForClaude(ctx context.Context, containerName string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if m.claudeInstalled(ctx, containerName) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
