package minimax

import (
	"context"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestCapabilitiesExposeMiniMaxM3ThinkingSwitch(t *testing.T) {
	caps, err := (&Provider{}).Capabilities(context.Background(), agent.CapabilityRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if caps.Provider != agent.ProviderMiniMax || caps.DefaultMode != agent.RunModeDefault {
		t.Fatalf("capabilities = %#v", caps)
	}
	if len(caps.Models) != 2 || caps.Models[0].ID != "" || caps.Models[1].ID != miniMaxModel {
		t.Fatalf("models = %#v", caps.Models)
	}
	model := caps.Models[1]
	if !model.ProviderDefault || model.DefaultReasoningEffort != "high" || len(model.ReasoningEfforts) != 3 {
		t.Fatalf("MiniMax model = %#v", model)
	}
	if model.ReasoningEfforts[1].Value != "none" || model.ReasoningEfforts[2].Value != "high" {
		t.Fatalf("reasoning options = %#v", model.ReasoningEfforts)
	}
	if len(caps.Modes) != 2 || caps.Modes[1].Value != string(agent.RunModePlan) {
		t.Fatalf("modes = %#v", caps.Modes)
	}
}

func TestMiniMaxReasoningEffortUsesDocumentedBinarySwitch(t *testing.T) {
	for input, want := range map[agent.ReasoningEffort]agent.ReasoningEffort{
		"":       "high",
		"none":   "none",
		"NONE":   "none",
		"medium": "high",
		"high":   "high",
	} {
		if got := miniMaxReasoningEffort(input); got != want {
			t.Fatalf("miniMaxReasoningEffort(%q) = %q, want %q", input, got, want)
		}
	}
}
