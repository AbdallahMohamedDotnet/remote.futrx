# Base Image: `futrx-remote-dev-base`

Every project container is launched from a custom LXD image, **`futrx-remote-dev-base`**, baked once on the host. Keeping the image preinstalled with Node and the registered agent CLIs means container creation takes ~5s instead of ~90s, and the first prompt is no longer dominated by an apt/npm install.

## What the image contains

- Ubuntu 24.04 (from `ubuntu:24.04`)
- Node.js 22 (via `deb.nodesource.com/setup_22.x`)
- `@anthropic-ai/claude-code` (repository-pinned version)
- `@openai/codex` (repository-pinned version)
- `@moonshot-ai/kimi-code` (provider-pinned version)

The provider-neutral recipe and shared version manifest are composed from the agent catalog:

- [`internal/service/agent_catalog.go`](../backend/internal/service/agent_catalog.go) — the Claude, Codex, and Kimi registration catalog supplied to image builds and runtime containers.
- [`internal/integration/containers/baseimage.go`](../backend/internal/integration/containers/baseimage.go) — generates the image recipe from those profiles.
- [`internal/agent/provisioning/agent-cli-versions.env`](../backend/internal/agent/provisioning/agent-cli-versions.env) — the Claude Code and Codex pins used by the image, runtime repair, and host installer. (Host Node/Go pins live separately in [`infra/versions.env`](../infra/versions.env).)

Each provider applies its own profile before a prompt. Claude and Codex compare their installed versions with the repository pin; Kimi checks for its binary. A current container only pays for a local check, while a missing or stale CLI is repaired in place before the agent starts.

## Building / rebuilding the image

A self-contained Go binary, `cmd/build-base-image`, drives the whole flow: launch a fresh `ubuntu:24.04` builder, run the install script, publish under the alias, delete the builder. Re-run it any time you bump a dependency.

```bash
cd backend

# First-time build (no existing alias):
go run ./cmd/build-base-image

# Rebuild after bumping Node or an agent CLI:
go run ./cmd/build-base-image -overwrite

# Custom alias (for testing a new image without disrupting production):
go run ./cmd/build-base-image -alias futrx-remote-dev-base-test
```

Requirements on the host: the `lxc` CLI on PATH, network access to `deb.nodesource.com` + npm, ~700 MB of free LXD storage for the published image.

Typical runtime: 60-120s. The CLI logs progress and prints the published alias on success.

## Effect on existing containers

Rebuilding the image immediately affects **new** containers. Existing containers keep their old rootfs, but registered providers repair their CLI the next time they run. You can still force convergence operationally:

| Path | Effect | Command |
| --- | --- | --- |
| **Automatic** | Existing containers repair a missing or stale registered agent CLI on the next prompt. | Nothing — deploy the backend and use the provider. |
| **Force-recreate** | Wipe an old project container; the next project start re-launches it from the new image. Workspace files are bind-mounted so they survive; anything custom in the rootfs is gone. | `lxc delete --force <project-container>` |
| **Manual in-place upgrade** | Upgrade immediately instead of waiting for the next prompt. | `lxc exec <container> -- npm install -g @openai/codex@<pin> @anthropic-ai/claude-code@<pin> @moonshot-ai/kimi-code@0.19.2` |

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
