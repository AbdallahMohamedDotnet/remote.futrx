package image

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// baseImageInstallPreamble is the provider-neutral part of the shell recipe.
// Agent packages contribute the npm packages and binaries appended below.
const baseImageInstallPreamble = `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq

# Core build / shell / network deps, plus git + ssh + jq for everyday agent
# work. python3-pip pulls in python3 too. Skip wrangler/aws/gcloud/hcloud —
# project-specific; agent installs them on demand and records in /workspace/setup.sh.
apt-get install -y -qq \
    curl ca-certificates gnupg \
    git openssh-client \
    jq build-essential python3-pip

# Node 22 (provides node + npm + npx for agent CLIs and JS tooling).
curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null 2>&1
apt-get install -y -qq nodejs

# Official GitHub CLI repo. Auth comes from $GITHUB_TOKEN at runtime,
# pushed per-project from the Secrets UI.
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    > /etc/apt/sources.list.d/github-cli.list
apt-get update -qq
apt-get install -y -qq gh`

// InstallScript generates the provider-neutral base recipe plus every
// configured agent CLI at its configured version.
func InstallScript(profiles []provisioning.Profile) (string, error) {
	packages := make([]string, 0, len(profiles))
	binaries := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile.CLI.PackageName == "" || profile.CLI.Binary == "" {
			return "", fmt.Errorf("agent profile %q has an incomplete CLI definition", profile.ID)
		}
		packages = append(packages, profile.CLI.NPMPackage())
		binaries = append(binaries, profile.CLI.Binary)
	}
	if len(packages) == 0 {
		return "", errors.New("no agent profiles configured")
	}

	var script strings.Builder
	script.WriteString(baseImageInstallPreamble)
	script.WriteString("\n\n# Agent CLIs.\nnpm install -g ")
	script.WriteString(strings.Join(packages, " "))
	script.WriteString(" --silent 2>&1 | tail -8\n\n# Sanity check the full toolchain.\nwhich ")
	script.WriteString(strings.Join(binaries, " "))
	script.WriteString(" git gh jq node npm python3 ssh\n")
	for _, binary := range binaries {
		script.WriteString(binary)
		script.WriteString(" --version\n")
	}
	script.WriteString("node --version\ngh --version | head -1")
	return script.String(), nil
}

func description(profiles []provisioning.Profile) string {
	labels := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile.CLI.ImageLabel != "" {
			labels = append(labels, profile.CLI.ImageLabel)
		}
	}
	description := "futrx remote dev base: ubuntu 24.04 + node 22"
	if len(labels) > 0 {
		description += " + " + strings.Join(labels, " + ")
	}
	return description
}
