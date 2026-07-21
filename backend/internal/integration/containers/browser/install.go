package browser

import (
	_ "embed"
	"strings"
)

//go:embed assets/agent-browser-install.sh
var embeddedAgentBrowserInstallScript string

// agentBrowserInstallScript installs the headed-browser stack used by the
// Agent Browser feature. It is shared by base-image builds and the on-demand
// repair path for older containers.
func InstallScript() string {
	// The former raw string literal had no trailing newline. Preserve its exact
	// command argument while keeping the shell program in its natural layer.
	return strings.TrimSuffix(embeddedAgentBrowserInstallScript, "\n")
}
