package containers

// Base-image provisioning. The same install script is used in two places:
//   1. As the recipe baked into the published futrx-remote-dev-base LXD image
//      (run once by cmd/build-base-image on a fresh ubuntu:24.04 builder).
//   2. As the fallback in EnsureClaude — runs inside an already-running
//      container if the CLI is missing (covers older proj-* containers that
//      pre-date the custom image).
// Keeping it in one constant guarantees the two paths can never drift.

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// BaseImageAlias is the LXD image alias the manager launches by default.
	BaseImageAlias = "futrx-remote-dev-base"

	// BaseImageSourceImage is the upstream image used as the builder rootfs
	// when (re)building BaseImageAlias.
	BaseImageSourceImage = "ubuntu:24.04"

	// BaseImageDescription is attached to the published image.
	BaseImageDescription = "futrx remote dev base: ubuntu 24.04 + node 20 + @anthropic-ai/claude-code"

	// baseImageBuilderName is the name used for the throwaway builder
	// container. Kept stable so a retry can clean up a leftover builder
	// from a previous failed run.
	baseImageBuilderName = "futrx-remote-dev-builder"

	baseImageBuildTimeout   = 15 * time.Minute
	baseImagePublishTimeout = 5 * time.Minute
	baseImageNetworkWarmup  = 3 * time.Second
)

// BaseImageInstallScript is the shell recipe that turns a fresh
// ubuntu:24.04 rootfs into the futrx-remote-dev-base image. It is also the
// fallback EnsureClaude runs inside an already-launched container.
const BaseImageInstallScript = `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq

# Core build / shell / network deps, plus git + ssh + jq for everyday agent
# work. python3-pip pulls in python3 too. Skip wrangler/aws/gcloud/hcloud —
# project-specific; agent installs them on demand and records in /workspace/setup.sh.
apt-get install -y -qq \
    curl ca-certificates gnupg \
    git openssh-client \
    jq build-essential python3-pip

# Node 20 (provides node + npm + npx for the Claude CLI and any JS tooling).
curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null 2>&1
apt-get install -y -qq nodejs

# Official GitHub CLI repo. Auth comes from $GITHUB_TOKEN at runtime,
# pushed per-project from the Secrets UI.
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    > /etc/apt/sources.list.d/github-cli.list
apt-get update -qq
apt-get install -y -qq gh

# Anthropic Claude CLI.
npm install -g @anthropic-ai/claude-code --silent 2>&1 | tail -3

# Sanity check the full toolchain.
which claude git gh jq node npm python3 ssh
claude --version
node --version
gh --version | head -1`

// BuildBaseImage launches a fresh BaseImageSourceImage container, runs the
// install script, publishes the result under alias, and removes the
// builder. If alias is empty, BaseImageAlias is used.
//
// Any previous publish at alias is NOT removed automatically — callers that
// want a clean overwrite should delete the image first via
// `lxc image delete <alias>`.
func (m *Manager) BuildBaseImage(ctx context.Context, alias string) error {
	if !m.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}
	if alias == "" {
		alias = BaseImageAlias
	}

	// Clean up any leftover builder from a previous interrupted run before
	// we try to launch a fresh one. Best-effort; failures are tolerated
	// because the container may simply not exist.
	cleanCtx, cleanCancel := context.WithTimeout(ctx, deleteTimeout)
	_, _ = m.lxc.Run(cleanCtx, "delete", "--force", baseImageBuilderName)
	cleanCancel()

	bctx, bcancel := context.WithTimeout(ctx, baseImageBuildTimeout)
	defer bcancel()

	// Deferred best-effort cleanup. Runs on every exit path including
	// success — once we have published, the builder is no longer needed.
	defer func() {
		dctx, dcancel := context.WithTimeout(context.Background(), deleteTimeout)
		defer dcancel()
		_, _ = m.lxc.Run(dctx, "delete", "--force", baseImageBuilderName)
	}()

	if out, err := m.lxc.Run(bctx, "launch", BaseImageSourceImage, baseImageBuilderName); err != nil {
		return fmt.Errorf("launch builder: %w; output: %s", err, out)
	}

	// Give cloud-init / systemd-resolved a moment so apt-get can reach the
	// network on the first try.
	select {
	case <-time.After(baseImageNetworkWarmup):
	case <-bctx.Done():
		return bctx.Err()
	}

	if out, err := m.lxc.Run(bctx, "exec", baseImageBuilderName, "--", "bash", "-c", BaseImageInstallScript); err != nil {
		return fmt.Errorf("install script: %w; output: %s", err, truncateOut(out, 2000))
	}

	if out, err := m.lxc.Run(bctx, "stop", baseImageBuilderName); err != nil {
		return fmt.Errorf("stop builder: %w; output: %s", err, out)
	}

	pctx, pcancel := context.WithTimeout(ctx, baseImagePublishTimeout)
	defer pcancel()
	if out, err := m.lxc.Run(pctx, "publish", baseImageBuilderName,
		"--alias", alias,
		"description="+BaseImageDescription); err != nil {
		return fmt.Errorf("publish: %w; output: %s", err, out)
	}

	return nil
}
