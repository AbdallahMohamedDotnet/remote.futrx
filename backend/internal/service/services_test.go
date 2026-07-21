package service

import (
	"slices"
	"testing"
)

func TestAgentProfilesComeFromRegistrationCatalog(t *testing.T) {
	profiles := AgentProfiles()
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
		if profile.CLI.Binary == "" || profile.CLI.PackageName == "" {
			t.Fatalf("profile %q has incomplete CLI policy: %#v", profile.ID, profile.CLI)
		}
		if profile.Credentials.Empty() {
			t.Fatalf("profile %q has no credential policy", profile.ID)
		}
	}
	if want := []string{"claude", "codex", "kimi"}; !slices.Equal(ids, want) {
		t.Fatalf("profile IDs = %v, want %v", ids, want)
	}
}

func TestAgentProfilesReturnsDefensiveCopies(t *testing.T) {
	first := AgentProfiles()
	first[0].Credentials.Files[0].HostPath = "/changed"
	first[0].BrowserMCPTemplates[0].Content[0] = 'x'

	second := AgentProfiles()
	if second[0].Credentials.Files[0].HostPath == "/changed" {
		t.Fatal("credential policy mutation escaped the catalog")
	}
	if second[0].BrowserMCPTemplates[0].Content[0] == 'x' {
		t.Fatal("template mutation escaped the catalog")
	}
}
