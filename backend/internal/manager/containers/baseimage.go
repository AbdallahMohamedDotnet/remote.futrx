package containers

// Base-image provisioning. The same install script is used in two places:
//   1. As the recipe baked into the published futrx-remote-dev-base LXD image
//      (run once by cmd/build-base-image on a fresh ubuntu:24.04 builder).
//   2. As the fallback in EnsureClaude — runs inside an already-running
//      container if Claude is missing (covers older proj-* containers that
//      pre-date the custom image). Codex is required from the base image so
//      prompt runs do not trigger apt/npm installs.
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
	BaseImageDescription = "futrx remote dev base: ubuntu 24.04 + node 20 + claude-code + codex"

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
// fallback EnsureClaude run inside an already-launched container.
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

# Agent CLIs.
npm install -g @anthropic-ai/claude-code @openai/codex --silent 2>&1 | tail -8

# Sanity check the full toolchain.
which claude codex git gh jq node npm python3 ssh
claude --version
codex --version
node --version
gh --version | head -1`

// BrowserGUIInstallScript installs the headed-browser GUI stack used by the
// Agent Browser feature: a real Google Chrome rendered on a virtual display
// (Xvfb), shared with the user over noVNC and driven by the agent over CDP.
// Like BaseImageInstallScript it runs in two places so the two paths cannot
// drift:
//  1. Layered onto the published base image by BuildBaseImage.
//  2. As the on-demand fallback in EnsureBrowserGUI, for containers that
//     pre-date the GUI stack being baked into the image.
const BrowserGUIInstallScript = `set -e
export DEBIAN_FRONTEND=noninteractive
# Wait for the apt/dpkg lock rather than failing if apt-daily / unattended-
# upgrades is mid-run — a common race shortly after a container boots.
APT="apt-get -o DPkg::Lock::Timeout=300"
$APT update -qq

# Virtual display + VNC bridge (x11vnc -> websockify/noVNC over HTTP/WS), a
# lightweight window manager (openbox) so the browser window keeps stable
# input focus, and xdotool to activate that window. Font packages cover
# common web / CJK / emoji glyphs so real pages render legibly.
$APT install -y -qq \
    xvfb x11vnc novnc websockify openbox xdotool \
    libgtk-3-0t64 libgbm1 libasound2t64 libnss3 libxshmfence1 \
    dbus-x11 fonts-liberation fonts-noto-core fonts-noto-color-emoji

# Browser: Playwright's Chromium (Chrome for Testing), NOT google-chrome-stable.
# In an unprivileged LXC container google-chrome-stable's network service
# cannot open the remote-debugging socket (CreatePlatformSocket: EPERM), so the
# agent could never attach over CDP; the Chromium build can. It installs into
# /root/.cache/ms-playwright, where gui-up.sh and browser.mjs both find it.
npx --yes playwright@1.60.0 install chromium 2>&1 | tail -3

# Sanity check the GUI toolchain (the chromium glob fails the build if absent).
which Xvfb x11vnc websockify openbox
ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome`

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

	// Layer the headed-browser GUI stack on top (Agent Browser feature).
	if out, err := m.lxc.Run(bctx, "exec", baseImageBuilderName, "--", "bash", "-c", BrowserGUIInstallScript); err != nil {
		return fmt.Errorf("browser GUI install script: %w; output: %s", err, truncateOut(out, 2000))
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
