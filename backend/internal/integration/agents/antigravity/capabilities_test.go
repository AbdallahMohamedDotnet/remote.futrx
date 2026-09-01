package antigravity

import (
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestParseCapabilitiesFromCLIOutput(t *testing.T) {
	models := `Fetching available models...
gemini-3.7-flash-high     Gemini 3.7 Flash (High)
gemini-3.7-flash-medium   Gemini 3.7 Flash (Medium)
gemini-3.1-pro-high       Gemini 3.1 Pro (High)
claude-sonnet-4-6         Claude Sonnet 4.6 (Thinking)
`
	help := `
  --effort Reasoning effort (low|medium|high)
  --mode Agent mode (accept-edits, plan)
`
	caps := parseCLIOutputCatalog(models, help)
	if len(caps.Models) != 5 || caps.Models[1].ID != "gemini-3.7-flash-high" || caps.Models[4].ID != "claude-sonnet-4-6" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if caps.Models[1].Label != "Gemini 3.7 Flash (High)" || caps.Models[4].Label != "Claude Sonnet 4.6 (Thinking)" {
		t.Fatalf("model labels = %+v", caps.Models)
	}
	if got := caps.Models[0].ReasoningEfforts; len(got) != 4 || got[3].Value != "high" {
		t.Fatalf("automatic-model reasoning efforts = %+v", got)
	}
	if got := caps.Models[1].ReasoningEfforts; len(got) != 0 {
		t.Fatalf("variant slug must not advertise a second effort selector: %+v", got)
	}
	if len(caps.Modes) != 1 || caps.Modes[0].Value != string(agent.RunModeDefault) {
		t.Fatalf("modes = %+v", caps.Modes)
	}
}

func TestParseCapabilitiesFromJSONPrefersStableModelID(t *testing.T) {
	models := `{"command":{"name":"models","data":{"models":[
  {"id":"gemini-stable-id","label":"Gemini Display Name"},
  {"id":"claude-sonnet-4-6","label":"Claude Sonnet 4.6 (Thinking)"},
  {"id":"gemini-stable-id","label":"Duplicate"}
]}}}`

	caps := parseCLIOutputCatalog(models, "")
	if len(caps.Models) != 3 {
		t.Fatalf("models = %+v", caps.Models)
	}
	if got := caps.Models[1]; got.ID != "gemini-stable-id" || got.Label != "Gemini Display Name" {
		t.Fatalf("stable model = %+v", got)
	}
	if got := caps.Models[2]; got.ID != "claude-sonnet-4-6" || got.Label != "Claude Sonnet 4.6 (Thinking)" {
		t.Fatalf("second model = %+v", got)
	}
}

func TestParseCapabilitiesRejectsUnrelatedJSONEnvelope(t *testing.T) {
	caps := parseCLIOutputCatalog(`{"command":{"name":"status","data":{"models":[{"id":"not-a-model","label":"No"}]}}}`, "")
	if len(caps.Models) != 1 || caps.Models[0].ID != "" {
		t.Fatalf("unexpected models = %+v", caps.Models)
	}
	if caps.Source != agent.CapabilitySourceFallback || caps.Warning == "" {
		t.Fatalf("malformed successful discovery was treated as live: %+v", caps)
	}
}
