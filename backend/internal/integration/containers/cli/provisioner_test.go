package cli

import (
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

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
