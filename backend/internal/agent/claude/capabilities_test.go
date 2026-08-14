package claude

import "testing"

func TestParseCapabilitiesFromHelp(t *testing.T) {
	help := `
  --effort <level>  Effort level for the current session (low, medium, high, xhigh, max)
  --model <model>   Provide an alias for the latest model (e.g. 'fable', 'opus', or 'sonnet')
  --permission-mode <mode> Permission mode (choices: "acceptEdits", "auto", "plan")
`
	caps := parseCapabilities(help)
	if caps.Source != "live" || len(caps.Models) != 4 {
		t.Fatalf("capabilities = %+v", caps)
	}
	if caps.Models[1].ID != "fable" || caps.Models[1].ReasoningEfforts[5].Value != "max" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if !caps.Modes[1].Native {
		t.Fatalf("plan mode should be native: %+v", caps.Modes)
	}
}

func TestParseCapabilitiesFallsBackForIncompleteHelp(t *testing.T) {
	caps := parseCapabilities("--model <model>")
	if caps.Source != "fallback" || len(caps.Models) < 2 || caps.Warning == "" {
		t.Fatalf("capabilities = %+v", caps)
	}
}
