package kimi

import "testing"

func TestParseCapabilitiesFromProviderJSON(t *testing.T) {
	caps, err := parseProviderCatalog([]byte(`{
  "providers": {"custom": {}},
  "models": {"moonshot/kimi-k2": {}, "fast": {}}
}`), "--plan Start in plan mode")
	if err != nil {
		t.Fatal(err)
	}
	if len(caps.Models) != 3 || caps.Models[1].ID != "fast" || caps.Models[2].ID != "moonshot/kimi-k2" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if !caps.Modes[1].Native {
		t.Fatalf("plan mode should be native: %+v", caps.Modes)
	}
}

func TestParseCapabilitiesAllowsEmptyBuiltInCatalog(t *testing.T) {
	caps, err := parseProviderCatalog([]byte(`{"providers":{},"models":{}}`), "")
	if err != nil || len(caps.Models) != 1 || caps.Models[0].ID != "" {
		t.Fatalf("capabilities = %+v, err = %v", caps, err)
	}
}
