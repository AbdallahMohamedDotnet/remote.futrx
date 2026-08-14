package chat

import (
	"encoding/json"
	"testing"
)

func TestCapabilityValuesRemainProviderDefined(t *testing.T) {
	if got := NormalizeReasoningEffort(" Future.V2 "); got != "Future.V2" {
		t.Fatalf("reasoning effort = %q", got)
	}
	if got := NormalizeServiceTier(" Burst_2 "); got != "Burst_2" {
		t.Fatalf("service tier = %q", got)
	}
	if got := NormalizeServiceTier("unsafe;value"); got != "" {
		t.Fatalf("unsafe service tier = %q", got)
	}
}

func TestMetaJSONPreservesExplicitAutoSelections(t *testing.T) {
	raw, err := json.Marshal(Meta{ID: "abcd"})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"model", "mode", "reasoningEffort", "serviceTier"} {
		value, ok := result[field]
		if !ok || value != "" {
			t.Fatalf("%s = %#v, present = %t", field, value, ok)
		}
	}
}
