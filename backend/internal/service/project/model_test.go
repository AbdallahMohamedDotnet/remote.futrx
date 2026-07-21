package project

import "testing"

func TestSetAgentStatusesPreservesLegacyInspectFields(t *testing.T) {
	var inspect ContainerInspect
	inspect.SetAgentStatuses([]AgentContainerStatus{
		{
			ID:                    "claude",
			Installed:             true,
			Version:               "2.1.206",
			InstructionsInstalled: true,
			InstructionsInSync:    true,
		},
		{ID: "codex", Installed: true, Version: "0.144.1"},
		{ID: "kimi", Installed: true, Version: "0.19.2"},
	})

	if !inspect.Claude.Installed || inspect.Claude.Version != "2.1.206" ||
		!inspect.Claude.ClaudeMDInstalled || !inspect.Claude.ClaudeMDInSync {
		t.Fatalf("Claude compatibility status = %#v", inspect.Claude)
	}
	if !inspect.Codex.Installed || inspect.Codex.Version != "0.144.1" {
		t.Fatalf("Codex compatibility status = %#v", inspect.Codex)
	}
}
