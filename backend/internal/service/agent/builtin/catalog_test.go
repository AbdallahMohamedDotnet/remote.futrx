package builtin

import (
	"slices"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/service/agent/module"
)

func TestCatalogBuildsEveryDeclaredAgentInStableOrder(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	descriptors := catalog.Descriptors()
	ids := make([]agent.ProviderID, len(descriptors))
	for index, descriptor := range descriptors {
		ids[index] = descriptor.ID
		if descriptor.Profile == nil || descriptor.Profile.ID != string(descriptor.ID) {
			t.Fatalf("descriptor %q has profile %#v", descriptor.ID, descriptor.Profile)
		}
	}
	want := []agent.ProviderID{
		agent.ProviderClaude,
		agent.ProviderCodex,
		agent.ProviderKimi,
		agent.ProviderAntigravity,
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("agent order = %v, want %v", ids, want)
	}

	runtime, err := catalog.Build(agentmodule.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range descriptors {
		provider := runtime.Providers.Lookup(descriptor.ID)
		if provider == nil || provider.ID() != descriptor.ID {
			t.Fatalf("provider %q was not built consistently", descriptor.ID)
		}
		binding, ok := runtime.Auth.Lookup(descriptor.ID)
		if !ok || binding.ID() != descriptor.ID {
			t.Fatalf("auth binding %q was not built consistently", descriptor.ID)
		}
	}
}

func TestCatalogProfilesAreDefensiveCopies(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	first := catalog.Profiles()
	first[0].Credentials.Files[0].HostPath = "/changed"
	first[0].BrowserMCPTemplates[0].Content[0] = 'x'

	second := catalog.Profiles()
	if second[0].Credentials.Files[0].HostPath == "/changed" {
		t.Fatal("credential policy mutation escaped the catalog")
	}
	if second[0].BrowserMCPTemplates[0].Content[0] == 'x' {
		t.Fatal("template mutation escaped the catalog")
	}
}
