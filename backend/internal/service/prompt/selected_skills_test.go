package prompt

import (
	"strings"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

func TestPromptWithSelectedSkillsPrefixesClaudeSlashCommands(t *testing.T) {
	got := promptWithSelectedSkills(agent.ProviderClaude, []servicechat.SkillRef{
		{Name: "Frontend Design", Command: "frontend-design", Provider: servicechat.ProviderClaude},
	}, "build the UI")

	want := "/frontend-design\n\nbuild the UI"
	if got != want {
		t.Fatalf("prompt = %q, want %q", got, want)
	}
}

func TestPromptWithSelectedSkillsPrefixesCodexDollarTriggers(t *testing.T) {
	got := promptWithSelectedSkills(agent.ProviderCodex, []servicechat.SkillRef{
		{Name: "Frontend Design", Command: "frontend-design", Provider: servicechat.ProviderCodex},
		{Name: "Review", Command: "review", Provider: servicechat.ProviderCodex},
	}, "build the UI")

	if !strings.HasPrefix(got, "Use these Codex skills for this request: $frontend-design $review\n\n") {
		t.Fatalf("missing codex skill prefix: %q", got)
	}
	if !strings.HasSuffix(got, "build the UI") {
		t.Fatalf("missing original prompt: %q", got)
	}
}

func TestPromptWithSelectedSkillsFiltersOtherProviders(t *testing.T) {
	got := promptWithSelectedSkills(agent.ProviderCodex, []servicechat.SkillRef{
		{Name: "Claude Only", Command: "claude-only", Provider: servicechat.ProviderClaude},
	}, "ship it")

	if got != "ship it" {
		t.Fatalf("prompt = %q", got)
	}
}

func TestSkillTriggerNameFallsBackToSingleToken(t *testing.T) {
	got := promptWithSelectedSkills(agent.ProviderCodex, []servicechat.SkillRef{
		{Name: "Frontend Design", Provider: servicechat.ProviderCodex},
	}, "ship it")

	if !strings.Contains(got, "$Frontend-Design") {
		t.Fatalf("prompt = %q", got)
	}
}

func TestHasBrowserSkill(t *testing.T) {
	if !hasBrowserSkill([]servicechat.SkillRef{{Name: "Browser", Command: "browser"}}) {
		t.Fatal("expected browser skill command to enable browser tools")
	}
	if hasBrowserSkill([]servicechat.SkillRef{{Name: "Review", Command: "review"}}) {
		t.Fatal("non-browser skill should not enable browser tools")
	}
}
