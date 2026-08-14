package kimi

import (
	"slices"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestArgsUseNativePlanModeWhenSelected(t *testing.T) {
	provider := &Provider{}
	plan := provider.args(agent.RunRequest{Prompt: "inspect", Mode: "plan"})
	if !slices.Contains(plan, "--plan") {
		t.Fatalf("native Plan mode missing: %#v", plan)
	}

	defaults := provider.args(agent.RunRequest{Prompt: "implement", Mode: "default"})
	if slices.Contains(defaults, "--plan") {
		t.Fatalf("default mode unexpectedly enabled Plan: %#v", defaults)
	}
}
