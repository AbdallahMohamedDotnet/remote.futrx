package claude

import (
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestParseCapabilitiesFromHelp(t *testing.T) {
	help := `
  --effort <level>  Effort level for the current session (low, medium, high, xhigh, max)
  --model <model>   Provide an alias for the latest model (e.g. 'fable', 'opus', or 'sonnet')
  --permission-mode <mode> Permission mode (choices: "acceptEdits", "auto", "plan")
`
	caps := parseCapabilityHelp(help)
	if caps.Source != "live" || len(caps.Models) != 4 {
		t.Fatalf("capabilities = %+v", caps)
	}
	if caps.Models[1].ID != "fable" || caps.Models[1].ReasoningEfforts[5].Value != "max" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if got := caps.Models[0].ServiceTiers; len(got) != 2 || got[0].Value != "" || got[1].Value != fastServiceTier {
		t.Fatalf("auto speed tiers = %+v", got)
	}
	if got := caps.Models[2]; got.ID != "opus" || len(got.ServiceTiers) != 2 || got.ServiceTiers[1].Value != fastServiceTier {
		t.Fatalf("opus capabilities = %+v", got)
	}
	if got := caps.Models[1].ServiceTiers; len(got) != 0 {
		t.Fatalf("fable speed tiers = %+v", got)
	}
	if len(caps.Modes) != 2 || caps.Modes[0].Value != string(agent.RunModeDefault) || caps.Modes[1].Value != string(agent.RunModePlan) {
		t.Fatalf("modes = %+v", caps.Modes)
	}
}

func TestParseCapabilitiesFallsBackForIncompleteHelp(t *testing.T) {
	caps := parseCapabilityHelp("--model <model>")
	if caps.Source != "fallback" || len(caps.Models) < 2 || caps.Warning == "" {
		t.Fatalf("capabilities = %+v", caps)
	}
	if len(caps.Modes) != 2 || caps.Modes[0].Value != string(agent.RunModeDefault) || caps.Modes[1].Value != string(agent.RunModePlan) {
		t.Fatalf("fallback modes = %+v", caps.Modes)
	}
	if got := caps.Models[0].ServiceTiers; len(got) != 2 || got[1].Value != fastServiceTier {
		t.Fatalf("fallback auto speed tiers = %+v", got)
	}
	if got := caps.Models[2]; got.ID != "opus" || len(got.ServiceTiers) != 2 || got.ServiceTiers[1].Value != fastServiceTier {
		t.Fatalf("fallback opus capabilities = %+v", got)
	}
}
