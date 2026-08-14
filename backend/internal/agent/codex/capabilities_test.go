package codex

import (
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestBuildCapabilitiesPreservesPerModelControls(t *testing.T) {
	var models modelListResponse
	models.Data = append(models.Data, modelListItem{
		ID: "gpt-next", Model: "gpt-next", DisplayName: "GPT Next", IsDefault: true,
		DefaultReasoningEffort: "medium",
		SupportedReasoningEfforts: []reasoningEffortItem{{
			ReasoningEffort: "medium", Description: "balanced",
		}},
		ServiceTiers: []serviceTierItem{{ID: "priority", Name: "Fast", Description: "faster"}},
	})
	modes := collaborationModeListResponse{}
	modes.Data = append(modes.Data, collaborationModeItem{Name: "Plan", Mode: string(agent.RunModePlan)})

	caps := buildCapabilities(models, modes)
	if len(caps.Models) != 2 || caps.Models[0].ID != "" || caps.Models[1].ID != "gpt-next" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if got := caps.Models[1].ReasoningEfforts; len(got) != 2 || got[1].Value != "medium" {
		t.Fatalf("reasoning efforts = %+v", got)
	}
	if got := caps.Models[1].ServiceTiers; len(got) != 2 || got[1].Value != "priority" || got[1].Label != "Fast" {
		t.Fatalf("service tiers = %+v", got)
	}
	if len(caps.Modes) != 2 || caps.Modes[0].Value != string(agent.RunModeDefault) || caps.Modes[1].Value != string(agent.RunModePlan) {
		t.Fatalf("modes = %+v", caps.Modes)
	}
}
