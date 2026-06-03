package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestListSkillsFiltersBundled(t *testing.T) {
	// CLI-bundled skills live under .system and plugins/cache; the
	// picker should ignore both and only surface user-authored skills.
	agentsHome := t.TempDir()
	home := t.TempDir()
	writeSkill(t, filepath.Join(home, "skills", ".system", "openai-docs", "SKILL.md"), `---
name: "openai-docs"
description: "Use official OpenAI docs."
---
`)
	writeSkill(t, filepath.Join(agentsHome, "skills", "custom", "SKILL.md"), `# Custom Skill`)
	writeSkill(t, filepath.Join(home, "plugins", "cache", "plugin-a", "skills", "github", "SKILL.md"), `---
name: github
description: Triage GitHub work.
---
`)

	service := NewWithSkillHomes(agentsHome, t.TempDir(), home)
	got, err := service.List(context.Background(), ProviderCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the user-authored skill, got %#v", got)
	}
	if got[0].Name != "Custom Skill" || got[0].Command != "custom" || got[0].Source != "user" {
		t.Fatalf("unexpected skill metadata: %#v", got[0])
	}
}

func TestListSkillsUsesAgentsAsProjectSourceOfTruth(t *testing.T) {
	workspace := t.TempDir()
	writeSkill(t, filepath.Join(workspace, ".agents", "skills", "custom", "SKILL.md"), `# Custom Skill`)
	writeSkill(t, filepath.Join(workspace, ".claude", "skills", "custom", "SKILL.md"), `# Legacy Duplicate`)
	writeSkill(t, filepath.Join(workspace, ".claude", "skills", "legacy", "SKILL.md"), `# Legacy Skill`)

	service := NewWithSkillHomes(filepath.Join(t.TempDir(), "missing-agents"), filepath.Join(t.TempDir(), "missing-claude"), filepath.Join(t.TempDir(), "missing-codex"))
	got, err := service.List(context.Background(), ProviderClaude, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected canonical plus legacy fallback skills, got %#v", got)
	}
	if got[0].Command != "custom" || got[0].Name != "Custom Skill" {
		t.Fatalf("canonical skill should win duplicates, got %#v", got[0])
	}
	if got[1].Command != "legacy" {
		t.Fatalf("expected legacy fallback skill, got %#v", got[1])
	}
}

func TestListSkillsDedupesUserCompatibilityPaths(t *testing.T) {
	agentsHome := t.TempDir()
	codexHome := t.TempDir()
	writeSkill(t, filepath.Join(agentsHome, "skills", "custom", "SKILL.md"), `# Canonical Skill`)
	writeSkill(t, filepath.Join(codexHome, "skills", "custom", "SKILL.md"), `# Legacy Duplicate`)

	service := NewWithSkillHomes(agentsHome, t.TempDir(), codexHome)
	got, err := service.List(context.Background(), ProviderCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected duplicate user skill to be collapsed, got %#v", got)
	}
	if got[0].Name != "Canonical Skill" || got[0].Command != "custom" {
		t.Fatalf("canonical user skill should win duplicate, got %#v", got[0])
	}
}

func TestListMissingRootsReturnsEmptyList(t *testing.T) {
	service := NewWithSkillHomes(filepath.Join(t.TempDir(), "missing-agents"), filepath.Join(t.TempDir(), "missing-claude"), filepath.Join(t.TempDir(), "missing-codex"))
	got, err := service.List(context.Background(), ProviderClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty skill list, got %#v", got)
	}
}

func TestListRejectsInvalidProvider(t *testing.T) {
	service := NewWithHomes(t.TempDir(), t.TempDir())
	_, err := service.List(context.Background(), Provider("other"), "")
	if !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("expected invalid provider error, got %v", err)
	}
}

func writeSkill(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
