package codex

import "testing"

func TestBuildCapabilitiesPreservesPerModelControls(t *testing.T) {
	var models modelListResponse
	models.Data = append(models.Data, struct {
		ID                        string `json:"id"`
		Model                     string `json:"model"`
		DisplayName               string `json:"displayName"`
		Description               string `json:"description"`
		DefaultReasoningEffort    string `json:"defaultReasoningEffort"`
		DefaultServiceTier        string `json:"defaultServiceTier"`
		IsDefault                 bool   `json:"isDefault"`
		SupportedReasoningEfforts []struct {
			ReasoningEffort string `json:"reasoningEffort"`
			Description     string `json:"description"`
		} `json:"supportedReasoningEfforts"`
		ServiceTiers []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"serviceTiers"`
	}{
		ID: "gpt-next", Model: "gpt-next", DisplayName: "GPT Next", IsDefault: true,
		DefaultReasoningEffort: "medium",
		SupportedReasoningEfforts: []struct {
			ReasoningEffort string `json:"reasoningEffort"`
			Description     string `json:"description"`
		}{{ReasoningEffort: "medium", Description: "balanced"}},
		ServiceTiers: []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}{{ID: "priority", Name: "Fast", Description: "faster"}},
	})
	modes := collaborationModeListResponse{}
	modes.Data = append(modes.Data, struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}{Name: "Plan", Mode: "plan"})

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
	if !caps.Modes[1].Native {
		t.Fatalf("plan mode should be native: %+v", caps.Modes)
	}
}
