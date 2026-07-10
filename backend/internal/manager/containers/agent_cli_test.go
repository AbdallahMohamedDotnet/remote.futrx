package containers

import (
	"strings"
	"testing"
)

func TestPinnedAgentCLIVersions(t *testing.T) {
	if pinnedClaudeCodeVersion != "2.1.206" {
		t.Fatalf("Claude Code pin = %q", pinnedClaudeCodeVersion)
	}
	if pinnedCodexCLIVersion != "0.144.1" {
		t.Fatalf("Codex pin = %q", pinnedCodexCLIVersion)
	}
	for _, packageSpec := range []string{
		"@anthropic-ai/claude-code@" + pinnedClaudeCodeVersion,
		"@openai/codex@" + pinnedCodexCLIVersion,
	} {
		if !strings.Contains(BaseImageInstallScript, packageSpec) {
			t.Fatalf("base image install script is missing %q", packageSpec)
		}
	}
}

func TestSemanticVersionAtLeast(t *testing.T) {
	tests := []struct {
		name    string
		actual  string
		minimum string
		want    bool
	}{
		{name: "Codex output at pin", actual: "codex-cli 0.144.1", minimum: "0.144.1", want: true},
		{name: "Claude output above pin", actual: "2.1.207 (Claude Code)", minimum: "2.1.206", want: true},
		{name: "older patch", actual: "codex-cli 0.144.0", minimum: "0.144.1", want: false},
		{name: "same-core prerelease", actual: "codex-cli 0.144.1-alpha.2", minimum: "0.144.1", want: false},
		{name: "newer prerelease core", actual: "codex-cli 0.145.0-alpha.2", minimum: "0.144.1", want: true},
		{name: "unparseable", actual: "codex unknown", minimum: "0.144.1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semanticVersionAtLeast(tt.actual, tt.minimum); got != tt.want {
				t.Fatalf("semanticVersionAtLeast(%q, %q) = %v, want %v", tt.actual, tt.minimum, got, tt.want)
			}
		})
	}
}
