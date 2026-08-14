package claude

import (
	"slices"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestParseModelCatalogResultReturnsEverySelection(t *testing.T) {
	result := "Current model: Opus 4.8 (1M context) (default)\n" +
		"Usage: /model <name>. Available: sonnet, opus, haiku, fable, best, " +
		"sonnet[1m], opus[1m], fable[1m], opusplan, default, or a full model ID."

	defaultLabel, selections, err := parseModelCatalogResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if defaultLabel != "Opus 4.8 (1M context)" {
		t.Fatalf("default label = %q", defaultLabel)
	}
	want := []string{
		"sonnet", "opus", "haiku", "fable", "best",
		"sonnet[1m]", "opus[1m]", "fable[1m]", "opusplan",
	}
	if !slices.Equal(selections, want) {
		t.Fatalf("selections = %#v, want %#v", selections, want)
	}
}

func TestBuildCapabilitiesUsesResolvedVersionedLabels(t *testing.T) {
	catalog := claudeModelCatalog{
		Source:       agent.CapabilitySourceLive,
		DefaultLabel: "Opus 4.8 (1M context)",
		Selections: []claudeModelSelection{
			{ID: "fable", Label: "Fable 5", Description: "Claude Code selection: fable"},
			{ID: "opus", Label: "Opus 4.8", Description: "Claude Code selection: opus"},
			{ID: "opus[1m]", Label: "Opus 4.8 (1M context)", Description: "Claude Code selection: opus[1m]"},
		},
	}
	reasoning := parseHelpEfforts("--effort <level> (low, medium, high, xhigh, max)")
	caps := buildCapabilities(catalog, reasoning)

	if len(caps.Models) != 4 || caps.Models[0].Label != "Auto · Opus 4.8 (1M context)" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if caps.Models[1].ID != "fable" || caps.Models[1].Label != "Fable 5" {
		t.Fatalf("fable = %+v", caps.Models[1])
	}
	if got := caps.Models[1].ServiceTiers; len(got) != 0 {
		t.Fatalf("fable speed tiers = %+v", got)
	}
	for _, index := range []int{0, 2, 3} {
		if got := caps.Models[index].ServiceTiers; len(got) != 2 || got[1].Value != fastServiceTier {
			t.Fatalf("model %d speed tiers = %+v", index, got)
		}
	}
	if got := caps.Models[2].ReasoningEfforts; len(got) != 6 || got[5].Value != "max" {
		t.Fatalf("reasoning efforts = %+v", got)
	}
	if len(caps.Modes) != 2 || caps.Modes[0].Value != string(agent.RunModeDefault) || caps.Modes[1].Value != string(agent.RunModePlan) {
		t.Fatalf("modes = %+v", caps.Modes)
	}
}

func TestFallbackCapabilitiesKeepCompleteVersionedCatalog(t *testing.T) {
	caps := fallbackCapabilities()
	if caps.Source != agent.CapabilitySourceFallback || len(caps.Models) != 10 {
		t.Fatalf("capabilities = %+v", caps)
	}
	want := map[string]string{
		"fable":    "Fable 5",
		"opus":     "Opus 4.8",
		"sonnet":   "Sonnet 5",
		"haiku":    "Haiku 4.5",
		"opus[1m]": "Opus 4.8 (1M context)",
		"opusplan": "Opus 4.8 (Plan) · Sonnet 5 (Default)",
	}
	for _, model := range caps.Models {
		if label, ok := want[model.ID]; ok && model.Label != label {
			t.Fatalf("model %q label = %q, want %q", model.ID, model.Label, label)
		}
	}
}
