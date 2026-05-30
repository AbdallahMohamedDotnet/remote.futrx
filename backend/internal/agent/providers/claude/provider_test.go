package claude

import (
	"slices"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
)

func TestArgsUseDesktopLikeClaudeHeadlessMode(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{Model: "sonnet [1m]", ResumeID: "session-123"})

	want := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
		"--dangerously-skip-permissions",
		"--model", "sonnet",
		"--resume", "session-123",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
	if slices.Contains(args, "--bare") {
		t.Fatal("claude provider must not use --bare for desktop-like behavior")
	}
}
