package baseimage

import (
	"strings"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

func TestRecipeUsesConfiguredProfiles(t *testing.T) {
	profiles := []provisioning.Profile{
		{ID: "alpha", CLI: provisioning.CLISpec{ImageLabel: "alpha-cli", Binary: "alpha", PackageName: "@example/alpha", Version: "1.2.3"}},
		{ID: "beta", CLI: provisioning.CLISpec{ImageLabel: "beta-cli", Binary: "beta", PackageName: "@example/beta", Version: "4.5.6"}},
	}
	script, err := InstallScript(profiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@example/alpha@1.2.3", "@example/beta@4.5.6", "which alpha beta", "alpha --version", "beta --version"} {
		if !strings.Contains(script, want) {
			t.Fatalf("base image install script is missing %q", want)
		}
	}
	if got, want := description(profiles), "futrx remote dev base: ubuntu 24.04 + node 22 + alpha-cli + beta-cli"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}
