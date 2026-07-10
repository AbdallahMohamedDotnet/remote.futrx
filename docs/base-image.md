# Base Image: `futrx-remote-dev-base`

Every project container is launched from a custom LXD image, **`futrx-remote-dev-base`**, baked once on the host. Keeping the image preinstalled with Node + Claude + Codex means container creation takes ~5s instead of ~90s, and the first prompt is no longer dominated by an apt/npm install.

## What the image contains

- Ubuntu 24.04 (from `ubuntu:24.04`)
- Node.js 22 (via `deb.nodesource.com/setup_22.x`)
- `@anthropic-ai/claude-code` (repository-pinned version)
- `@openai/codex` (repository-pinned version)

The recipe and shared version manifest live in the container manager:

- [`internal/manager/containers/baseimage.go`](../backend/internal/manager/containers/baseimage.go) — the `BaseImageInstallScript` recipe.
- [`internal/manager/containers/agent-cli-versions.env`](../backend/internal/manager/containers/agent-cli-versions.env) — the Claude Code and Codex pins used by the image, runtime repair, and host installer.

Prompt runs compare the installed Claude Code or Codex version with the repository pin. A current container only pays for a local `--version` check; a missing or stale CLI is upgraded in place before the agent starts. This lets existing containers adopt model-compatibility updates without being recreated.

## Building / rebuilding the image

A self-contained Go binary, `cmd/build-base-image`, drives the whole flow: launch a fresh `ubuntu:24.04` builder, run the install script, publish under the alias, delete the builder. Re-run it any time you bump a dependency.

```bash
cd backend

# First-time build (no existing alias):
go run ./cmd/build-base-image

# Rebuild after bumping Node / Claude / Codex:
go run ./cmd/build-base-image -overwrite

# Custom alias (for testing a new image without disrupting production):
go run ./cmd/build-base-image -alias futrx-remote-dev-base-test
```

Requirements on the host: the `lxc` CLI on PATH, network access to `deb.nodesource.com` + npm, ~700 MB of free LXD storage for the published image.

Typical runtime: 60-120s. The CLI logs progress and prints the published alias on success.

## Effect on existing containers

Rebuilding the image immediately affects **new** containers. Existing containers keep their old rootfs, but Claude Code and Codex converge to the pinned versions the next time that provider runs. You can still force convergence operationally:

| Path | Effect | Command |
| --- | --- | --- |
| **Automatic** | Existing containers upgrade a stale Claude Code or Codex CLI on the next prompt. | Nothing — deploy the backend and use the provider. |
| **Force-recreate** | Wipe an old project container; the next project start re-launches it from the new image. Workspace files are bind-mounted so they survive; anything custom in the rootfs is gone. | `lxc delete --force <project-container>` |
| **Manual in-place upgrade** | Upgrade immediately instead of waiting for the next prompt. | `lxc exec <container> -- npm install -g @openai/codex@<pin> @anthropic-ai/claude-code@<pin>` |

## Bootstrap on a fresh host

Single command sequence to bring the application up on a new Linux server:

```bash
# 1. Install LXD (Ubuntu/Debian)
snap install lxd && lxd init --auto

# 2. Build the base image
cd backend && go run ./cmd/build-base-image

# 3. Start the server
go run ./cmd/remote
```

No manual `lxc launch / publish` shell ceremony — the binary owns it.
