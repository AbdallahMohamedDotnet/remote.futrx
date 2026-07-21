package containers

import (
	"strings"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

func TestBaseImageRecipeUsesConfiguredProfiles(t *testing.T) {
	profiles := []provisioning.Profile{
		{ID: "alpha", CLI: provisioning.CLISpec{ImageLabel: "alpha-cli", Binary: "alpha", PackageName: "@example/alpha", Version: "1.2.3"}},
		{ID: "beta", CLI: provisioning.CLISpec{ImageLabel: "beta-cli", Binary: "beta", PackageName: "@example/beta", Version: "4.5.6"}},
	}
	script, err := baseImageInstallScript(profiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@example/alpha@1.2.3", "@example/beta@4.5.6", "which alpha beta", "alpha --version", "beta --version"} {
		if !strings.Contains(script, want) {
			t.Fatalf("base image install script is missing %q", want)
		}
	}
	if got, want := baseImageDescription(profiles), "futrx remote dev base: ubuntu 24.04 + node 22 + alpha-cli + beta-cli"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestCLIInstallLabelReportsVersionOnlyWhenRequested(t *testing.T) {
	spec := provisioning.CLISpec{Name: "agent", Version: "1.2.3"}
	if got := cliInstallLabel(spec); got != "agent" {
		t.Fatalf("label without version = %q", got)
	}
	spec.ReportVersion = true
	if got := cliInstallLabel(spec); got != "agent 1.2.3" {
		t.Fatalf("label with version = %q", got)
	}
}

func TestSemanticVersionAtLeast(t *testing.T) {
	tests := []struct {
		name    string
		actual  string
		minimum string
		want    bool
	}{
		{name: "output at pin", actual: "agent-cli 0.144.1", minimum: "0.144.1", want: true},
		{name: "output above pin", actual: "2.1.207 (Agent CLI)", minimum: "2.1.206", want: true},
		{name: "older patch", actual: "agent-cli 0.144.0", minimum: "0.144.1", want: false},
		{name: "same-core prerelease", actual: "agent-cli 0.144.1-alpha.2", minimum: "0.144.1", want: false},
		{name: "newer prerelease core", actual: "agent-cli 0.145.0-alpha.2", minimum: "0.144.1", want: true},
		{name: "unparseable", actual: "agent unknown", minimum: "0.144.1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semanticVersionAtLeast(tt.actual, tt.minimum); got != tt.want {
				t.Fatalf("semanticVersionAtLeast(%q, %q) = %v, want %v", tt.actual, tt.minimum, got, tt.want)
			}
		})
	}
}
