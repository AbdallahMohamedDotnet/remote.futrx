package containers

// Claude-specific provider glue: installing the CLI inside a container and
// the AuthBundle that describes Claude's credentials. Anything generic
// (lifecycle, push/pull pipeline) lives in the other files; this file is
// where Claude knowledge is concentrated.

import (
	"context"
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

// EnsureClaude installs or upgrades Claude Code to the repository pin.
// Safe to call on every prompt: current containers only pay for a local
// version check, while stale containers self-heal in place.
func (m *Manager) EnsureClaude(ctx context.Context, containerName string) error {
	return m.ensureAgentCLI(ctx, containerName, claudeCLISpec)
}
