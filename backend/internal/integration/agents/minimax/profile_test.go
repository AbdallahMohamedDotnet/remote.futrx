package minimax

import (
	"encoding/json"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

func TestProfileUsesIsolatedCodexHomeAndOfficialModelCatalog(t *testing.T) {
	profile := Profile()
	if profile.ID != "minimax" || profile.CLI.Binary != "codex" ||
		profile.CLI.PackageName != "@openai/codex" ||
		profile.CLI.Version != provisioning.MustCLIVersion("CODEX_CLI_VERSION") {
		t.Fatalf("CLI profile = %#v", profile)
	}
	if len(profile.PersistentState) != 1 || profile.PersistentState[0] != (provisioning.PersistentDirectory{
		Device: "minimax-home", HostDirectory: "minimax", ContainerPath: "/root/.minimax",
	}) {
		t.Fatalf("persistent state = %#v", profile.PersistentState)
	}
	if profile.Instructions == nil || profile.Instructions.Path != "/root/.minimax/AGENTS.md" {
		t.Fatalf("instructions = %#v", profile.Instructions)
	}
	if profile.WorkspaceSkills == nil || profile.WorkspaceSkills.WorkspaceHome != "/workspace/.minimax" ||
		profile.WorkspaceSkills.HomeSkillsDir != "/root/.minimax/skills" {
		t.Fatalf("workspace skills = %#v", profile.WorkspaceSkills)
	}
	if len(profile.RuntimeTemplates) != 1 || profile.RuntimeTemplates[0].Path != containerMiniMaxCatalog {
		t.Fatalf("runtime templates = %#v", profile.RuntimeTemplates)
	}

	var catalog struct {
		Models []struct {
			Slug                       string   `json:"slug"`
			DefaultReasoningLevel      string   `json:"default_reasoning_level"`
			SupportsReasoningSummaries bool     `json:"supports_reasoning_summaries"`
			ShellType                  string   `json:"shell_type"`
			InputModalities            []string `json:"input_modalities"`
		} `json:"models"`
	}
	if err := json.Unmarshal(profile.RuntimeTemplates[0].Content, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) != 1 || catalog.Models[0].Slug != miniMaxModel ||
		catalog.Models[0].DefaultReasoningLevel != "high" ||
		!catalog.Models[0].SupportsReasoningSummaries ||
		catalog.Models[0].ShellType != "shell_command" || len(catalog.Models[0].InputModalities) != 2 {
		t.Fatalf("model catalog = %#v", catalog.Models)
	}
}

func TestProfileReturnsDefensiveCatalogCopies(t *testing.T) {
	first := Profile()
	first.RuntimeTemplates[0].Content[0] = 'x'
	if Profile().RuntimeTemplates[0].Content[0] == 'x' {
		t.Fatal("model catalog mutation escaped Profile")
	}
}
