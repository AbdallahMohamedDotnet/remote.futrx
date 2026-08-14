package chat

import "testing"

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
