// Package antigravity adapts Google's Antigravity CLI (`agy`) as a headless
// agent provider. The CLI has a print mode but no structured event stream, so
// runs surface as plain streaming text; conversation ids are recovered from
// the CLI's on-disk brain directory because print mode never emits them
// (github.com/google-antigravity/antigravity-cli issue #7).
package antigravity

import (
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

// containerAgentHome is HOME inside a project container: agy keeps its state
// (auth fallback tokens, conversation brain) under $HOME/.gemini.
const containerAgentHome = "/root"

// stateDirUnderHome is where agy stores conversations and headless auth state.
const stateDirUnderHome = ".gemini/antigravity-cli"

// manifestBaseURL serves per-platform release manifests: {version, url, sha512}.
const manifestBaseURL = "https://antigravity-cli-auto-updater-974169037036.us-central1.run.app"

var antigravityProfile = provisioning.Profile{
	ID: string(agent.ProviderAntigravity),
	CLI: provisioning.CLISpec{
		Name:               "antigravity",
		ImageLabel:         "antigravity",
		Binary:             "agy",
		Version:            provisioning.MustCLIVersion("ANTIGRAVITY_CLI_VERSION"),
		ReportVersion:      true,
		CheckVersion:       true,
		VerifyAfterInstall: true,
		InstallMode:        provisioning.InstallWithScript,
		InstallScript:      installScript(provisioning.MustCLIVersion("ANTIGRAVITY_CLI_VERSION")),
		InstallTimeout:     8 * time.Minute,
		WaitTimeout:        5 * time.Minute,
	},
	// No credential sync: agy stores auth in the OS keyring on desktops and in
	// per-home fallback files on headless systems, with no stable documented
	// token subpath to move between host and container. Sign-in is per
	// workspace: run `agy` once in the chat terminal and complete the URL +
	// code flow. Runs without credentials fail with agy's own sign-in message.
	Credentials: provisioning.CredentialSpec{Name: "antigravity"},
}

// Profile returns Antigravity's container-facing policy as a defensive copy.
func Profile() provisioning.Profile {
	return antigravityProfile.Clone()
}

// installScript downloads the pinned agy release for the container's
// architecture, verifies the manifest checksum, and installs /usr/local/bin/agy.
// It refuses to install a manifest version other than the repository pin so
// upgrades stay deliberate.
func installScript(version string) string {
	return fmt.Sprintf(`set -euo pipefail
if command -v agy >/dev/null 2>&1 && [ "$(agy --version 2>/dev/null)" = %[1]q ]; then
    exit 0
fi
case "$(uname -m)" in
    x86_64|amd64) platform="linux_amd64" ;;
    aarch64|arm64) platform="linux_arm64" ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
manifest=$(curl -fsSL "%[2]s/manifests/${platform}.json")
manifest_version=$(printf '%%s' "$manifest" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
url=$(printf '%%s' "$manifest" | sed -n 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
sha512=$(printf '%%s' "$manifest" | sed -n 's/.*"sha512"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ "$manifest_version" != %[1]q ]; then
    echo "antigravity manifest serves ${manifest_version}, repository pins %[1]s — update ANTIGRAVITY_CLI_VERSION" >&2
    exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/agy.tar.gz"
echo "${sha512}  $tmp/agy.tar.gz" | sha512sum -c - >/dev/null
tar -xzf "$tmp/agy.tar.gz" -C "$tmp" antigravity
install -m 0755 "$tmp/antigravity" /usr/local/bin/agy
agy --version`, version, manifestBaseURL)
}
