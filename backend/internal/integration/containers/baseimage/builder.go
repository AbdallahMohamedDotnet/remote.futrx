// Package baseimage builds the development image used by project containers.
package baseimage

// Base-image provisioning. The same install script is used in two places:
//   1. As the recipe baked into the published futrx-remote-dev-base LXD image
//      (run once by cmd/build-base-image on a fresh ubuntu:24.04 builder).
//   2. As the fallback when an already-running container predates Node/npm.
// Agent packages supply CLI definitions through provisioning profiles, so
// image builds and runtime repair use the same tested pins as prompt startup.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/output"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
)

const (
	// BaseImageAlias is the LXD image alias the client launches by default.
	Alias = "futrx-remote-dev-base"

	// BaseImageSourceImage is the upstream image used as the builder rootfs
	// when (re)building BaseImageAlias.
	SourceImage = "ubuntu:24.04"

	// baseImageBuilderName is the name used for the throwaway builder
	// container. Kept stable so a retry can clean up a leftover builder
	// from a previous failed run.
	baseImageBuilderName = "futrx-remote-dev-builder"

	baseImageBuildTimeout   = 15 * time.Minute
	baseImagePublishTimeout = 5 * time.Minute
	baseImageNetworkWarmup  = 3 * time.Second
	deleteTimeout           = 30 * time.Second
)

// baseImageBuilder owns the disposable builder lifecycle and publishes the
// profile-derived development image consumed by project containers.
type Builder struct {
	runner                  command.Runner
	profiles                *profiles.Registry
	browserInstallScript    string
	codeServerInstallScript []byte
}

// NewBuilder returns an image builder configured with the feature install
// programs layered onto the provider-neutral development image.
func NewBuilder(
	runner command.Runner,
	registry *profiles.Registry,
	browserInstallScript string,
	codeServerInstallScript []byte,
) *Builder {
	return &Builder{
		runner:                  runner,
		profiles:                registry,
		browserInstallScript:    browserInstallScript,
		codeServerInstallScript: codeServerInstallScript,
	}
}

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

// Build launches a fresh SourceImage container, runs the install script,
// publishes the result under alias, and removes the builder. If alias is
// empty, Alias is used.
//
// Any previous publish at alias is NOT removed automatically — callers that
// want a clean overwrite should delete the image first via
// `lxc image delete <alias>`.
func (b *Builder) Build(ctx context.Context, alias string) error {
	if !b.runner.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}
	if alias == "" {
		alias = Alias
	}
	profiles := b.profiles.Snapshot()
	installScript, err := InstallScript(profiles)
	if err != nil {
		return err
	}

	// Clean up any leftover builder from a previous interrupted run before
	// we try to launch a fresh one. Best-effort; failures are tolerated
	// because the container may simply not exist.
	cleanCtx, cleanCancel := context.WithTimeout(ctx, deleteTimeout)
	_, _ = b.runner.Run(cleanCtx, "delete", "--force", baseImageBuilderName)
	cleanCancel()

	bctx, bcancel := context.WithTimeout(ctx, baseImageBuildTimeout)
	defer bcancel()

	// Deferred best-effort cleanup. Runs on every exit path including
	// success — once we have published, the builder is no longer needed.
	defer func() {
		dctx, dcancel := context.WithTimeout(context.Background(), deleteTimeout)
		defer dcancel()
		_, _ = b.runner.Run(dctx, "delete", "--force", baseImageBuilderName)
	}()

	if out, err := b.runner.Run(bctx, "launch", SourceImage, baseImageBuilderName); err != nil {
		return fmt.Errorf("launch builder: %w; output: %s", err, out)
	}

	// Give cloud-init / systemd-resolved a moment so apt-get can reach the
	// network on the first try.
	select {
	case <-time.After(baseImageNetworkWarmup):
	case <-bctx.Done():
		return bctx.Err()
	}

	if out, err := b.runner.Run(bctx, "exec", baseImageBuilderName, "--", "bash", "-c", installScript); err != nil {
		return fmt.Errorf("install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	// Layer the headed-browser GUI stack on top (Agent Browser feature).
	if out, err := b.runner.Run(bctx, "exec", baseImageBuilderName, "--", "bash", "-c", b.browserInstallScript); err != nil {
		return fmt.Errorf("agent browser install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	// Layer the on-demand code-server IDE on top (per-container VS Code).
	if out, err := b.runner.Run(bctx, "exec", baseImageBuilderName, "--", "bash", "-c", string(b.codeServerInstallScript)); err != nil {
		return fmt.Errorf("code-server install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	if out, err := b.runner.Run(bctx, "stop", baseImageBuilderName); err != nil {
		return fmt.Errorf("stop builder: %w; output: %s", err, out)
	}

	pctx, pcancel := context.WithTimeout(ctx, baseImagePublishTimeout)
	defer pcancel()
	if out, err := b.runner.Run(pctx, "publish", baseImageBuilderName,
		"--alias", alias,
		"description="+description(profiles)); err != nil {
		return fmt.Errorf("publish: %w; output: %s", err, out)
	}

	return nil
}
