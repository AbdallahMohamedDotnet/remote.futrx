#!/usr/bin/env bash
# upgrade-workspaces.sh — converge every project container to the current
# base image.
#
# Flow:
#   1. Rebake futrx-remote-dev-base from the recipe (skip with --no-rebake,
#      e.g. when you just rebaked by hand).
#   2. Delete every idle project container. Recreation is automatic and
#      complete: on the next prompt or UI start, the backend sees the
#      container is missing and relaunches it from the new image, reattaches
#      the /workspace bind-mount, and re-provisions auth, code-server, env
#      and secrets (service/project.Start -> containers.Launch).
#
# Safety:
#   - Workspace files always survive (bind-mounted from the host). Anything
#     installed in the container ROOTFS outside /workspace (ad-hoc apt/npm
#     installs, caches) is lost — that is what "upgrade by re-clone" means.
#   - Containers with a running agent process (host-side `lxc exec <name> --`)
#     are SKIPPED by default so an in-flight chat is never killed. Re-run
#     later for the stragglers, or pass --include-busy to force them too.
#   - Only containers recorded in data/projects/*/meta.json are considered;
#     unrelated LXD containers on the same host are never touched.
#
# Usage:
#   sudo bash /opt/remote.futrx/infra/upgrade-workspaces.sh [flags]
#
# Flags:
#   --dry-run        show what would happen, change nothing
#   --no-rebake      skip step 1, only recycle containers
#   --include-busy   also delete containers with an active agent process
set -euo pipefail

INFRA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
PROJECTS_DIR="${FUTRX_DATA_DIR:-/opt/remote.futrx/data}/projects"

log()  { printf "\n\033[1;36m==> %s\033[0m\n" "$*"; }
warn() { printf "\033[1;33m!! %s\033[0m\n" "$*"; }
ok()   { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
err()  { printf "\n\033[1;31m✗ %s\033[0m\n" "$*" >&2; }

DRY_RUN=0
REBAKE=1
INCLUDE_BUSY=0
for a in "$@"; do
    case "$a" in
        --dry-run)      DRY_RUN=1 ;;
        --no-rebake)    REBAKE=0 ;;
        --include-busy) INCLUDE_BUSY=1 ;;
        *) err "unknown flag: $a"; exit 1 ;;
    esac
done

if [ "$EUID" -ne 0 ]; then
    err "needs root; rerun with sudo"
    exit 1
fi
if ! command -v lxc >/dev/null; then
    err "lxc CLI not found"
    exit 1
fi
if ! command -v jq >/dev/null; then
    err "jq CLI not found — run infra/update.sh or infra/install.sh first"
    exit 1
fi

# ───────────────── 1. rebake ─────────────────
if [ "$REBAKE" -eq 1 ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would rebake base image (go run ./cmd/build-base-image -overwrite)"
    else
        log "Rebaking base image (60-120s)"
        ( cd "$INFRA_DIR/../backend" && go run ./cmd/build-base-image -overwrite )
        ok "base image rebaked"
    fi
else
    log "Skipping rebake (--no-rebake) — recycling containers onto the existing image"
fi

# ───────────────── 2. recycle containers ─────────────────
log "Recycling project containers"
DELETED=0
SKIPPED_BUSY=0
project_container_names() {
    if [ ! -d "$PROJECTS_DIR" ]; then
        return
    fi
    find "$PROJECTS_DIR" -mindepth 2 -maxdepth 2 -type f -name meta.json -print0 \
        | xargs -0 -r jq -r '.containerName // empty' \
        | sort -u
}

while IFS= read -r name; do
    [ -n "$name" ] || continue
    if ! lxc info "$name" >/dev/null 2>&1; then
        continue
    fi

    # Host-side agent processes run as `lxc exec <name> -- ...` (claude,
    # codex, kimi). A live one means a chat is in flight — leave it alone.
    if [ "$INCLUDE_BUSY" -eq 0 ] && pgrep -f "lxc exec ${name} --" >/dev/null 2>&1; then
        warn "SKIP $name — active agent process (re-run later, or --include-busy)"
        SKIPPED_BUSY=$((SKIPPED_BUSY + 1))
        continue
    fi

    if [ "$DRY_RUN" -eq 1 ]; then
        ok "[dry-run] would delete $name"
    else
        lxc delete --force "$name"
        ok "deleted $name (relaunches from the new image on next use)"
    fi
    DELETED=$((DELETED + 1))
done < <(project_container_names)

# ───────────────── summary ─────────────────
echo
if [ "$DRY_RUN" -eq 1 ]; then
    ok "dry-run: $DELETED container(s) would be recycled, $SKIPPED_BUSY busy skipped"
else
    ok "$DELETED container(s) recycled, $SKIPPED_BUSY busy skipped"
    if [ "$SKIPPED_BUSY" -gt 0 ]; then
        warn "re-run this script once the busy workspaces go idle"
    fi
    echo "  Each workspace relaunches from the new image on its next prompt or UI start."
fi
