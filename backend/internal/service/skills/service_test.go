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
	home := t.TempDir()
	writeSkill(t, filepath.Join(home, "skills", ".system", "openai-docs", "SKILL.md"), `---
name: "openai-docs"
description: "Use official OpenAI docs."
---
`)
	writeSkill(t, filepath.Join(home, "skills", "custom", "SKILL.md"), `# Custom Skill`)
	writeSkill(t, filepath.Join(home, "plugins", "cache", "plugin-a", "skills", "github", "SKILL.md"), `---
name: github
description: Triage GitHub work.
---
`)

	service := NewWithHomes(t.TempDir(), home)
	got, err := service.List(context.Background(), ProviderCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the user-authored skill, got %#v", got)
	}
	if got[0].Name != "Custom Skill" || got[0].Source != "user" {
		t.Fatalf("unexpected skill: %#v", got[0])
	}
}

func TestListMissingRootsReturnsEmptyList(t *testing.T) {
	service := NewWithHomes(filepath.Join(t.TempDir(), "missing-claude"), filepath.Join(t.TempDir(), "missing-codex"))
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
