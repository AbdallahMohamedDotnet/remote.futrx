# Base Image: `futrx-remote-dev-base`

Every project container is launched from a custom LXD image, **`futrx-remote-dev-base`**, baked once on the host. Keeping the image preinstalled with Node + Claude + Codex means container creation takes ~5s instead of ~90s, and the first prompt is no longer dominated by an apt/npm install.

## What the image contains

- Ubuntu 24.04 (from `ubuntu:24.04`)
- Node.js 20 (via `deb.nodesource.com/setup_20.x`)
- `@anthropic-ai/claude-code` (latest at build time)
- `@openai/codex` (latest at build time)

The exact recipe lives in one place in the codebase:

- [`internal/manager/containers/baseimage.go`](../backend/internal/manager/containers/baseimage.go) — the `BaseImageInstallScript` constant.

That same constant is what `EnsureClaude` / `EnsureCodex` run as a fallback inside any container that was launched from an older image and is missing either CLI. The two paths can't drift.

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

Rebuilding the image only affects **new** containers (launched from this point forward). Existing containers keep whatever Node / agent CLIs they were created with. Three ways to bring them up to date — pick based on whether you care about preserving their non-`/workspace` state:

| Path | Effect | Command |
| --- | --- | --- |
| **Lazy** | New projects get the new image; old containers keep their old versions. | Nothing — just rebuild and move on. |
| **Force-recreate** | Wipe old containers; next prompt re-launches them from the new image. Workspace files are bind-mounted so they survive; anything custom in the rootfs is gone. | `lxc list -c n --format csv \| grep '^futrx-remote-dev-' \| xargs -I{} lxc delete --force {}` |
| **In-place upgrade** | Run the install script inside every existing container. Slow but preserves rootfs state. | `for c in $(lxc list -c n --format csv \| grep '^futrx-remote-dev-'); do lxc exec $c -- bash -c "$(cat <<-SH ... SH)" ; done` |

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
