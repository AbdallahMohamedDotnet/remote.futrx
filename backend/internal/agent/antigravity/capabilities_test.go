package antigravity

import "testing"

func TestParseCapabilitiesFromCLIOutput(t *testing.T) {
	models := `Available models:
- gemini-3-pro  Gemini 3 Pro
- google/gemini-fast  Gemini Fast
`
	help := `
  --effort Reasoning effort (low|medium|high)
  --mode Agent mode (accept-edits, plan)
`
	caps := parseCapabilities(models, help)
	if len(caps.Models) != 3 || caps.Models[1].ID != "gemini-3-pro" || caps.Models[2].ID != "google/gemini-fast" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if got := caps.Models[1].ReasoningEfforts; len(got) != 4 || got[3].Value != "high" {
		t.Fatalf("reasoning efforts = %+v", got)
	}
	if !caps.Modes[1].Native {
		t.Fatalf("plan mode should be native: %+v", caps.Modes)
	}
}
