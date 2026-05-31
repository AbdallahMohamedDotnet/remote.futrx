package codex

import (
	"slices"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
)

func TestArgsUseCodexExecJSONMode(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{Model: "gpt-5.5 [fast]"})

	want := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5.5",
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestArgsResumeThread(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{ResumeID: "thread-123"})

	want := []string{
		"exec",
		"resume",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"thread-123",
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}
