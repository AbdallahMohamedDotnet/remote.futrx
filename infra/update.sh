#!/usr/bin/env bash
# remote.futrx.dev — one-command update for an installed box.
#
# The updater deliberately pulls and re-executes itself before doing any
# convergence. That guarantees newly committed toolchain and agent CLI pins
# are used on this run, rather than requiring a second run after install.sh
# updates the checkout.
#
# Default flow:
#   1. Fast-forward /opt/remote.futrx.dev to origin/main.
#   2. Converge host dependencies and all host agent CLIs.
#   3. Build and restart the application.
#   4. Rebuild the base image with the pinned agent CLIs.
#   5. Recycle idle workspaces onto the new image (busy ones are skipped).
#
# Usage:
#   sudo bash /opt/remote.futrx.dev/infra/update.sh
#   sudo bash /opt/remote.futrx.dev/infra/update.sh <hostname>
#   sudo bash /opt/remote.futrx.dev/infra/update.sh --include-busy
#
# Flags:
#   --include-busy     also recycle workspaces with an active agent process
#   --skip-workspaces  update the host/application without rebuilding the base
#                      image or recycling workspace containers
set -euo pipefail

INSTALL_DIR="/opt/remote.futrx.dev"
UNIT="/etc/systemd/system/remote.futrx.dev.service"

usage() {
    sed -n '2,25s/^# \{0,1\}//p' "$0"
}

HOSTNAME=""
INCLUDE_BUSY=0
UPDATE_WORKSPACES=1
for a in "$@"; do
    case "$a" in
        --include-busy)    INCLUDE_BUSY=1 ;;
        --skip-workspaces) UPDATE_WORKSPACES=0 ;;
        -h|--help)         usage; exit 0 ;;
        --*)               echo "unknown flag: $a" >&2; exit 1 ;;
        *)
            if [ -n "$HOSTNAME" ]; then
                echo "unexpected argument: $a" >&2
                exit 1
            fi
            HOSTNAME="$a"
            ;;
    esac
done

if [ "$EUID" -ne 0 ]; then
    echo "this updater needs root; rerun with sudo" >&2
    exit 1
fi
if [ ! -d "$INSTALL_DIR/.git" ]; then
    echo "$INSTALL_DIR is not an installed git checkout; run infra/install.sh first" >&2
    exit 1
fi

# update.sh can itself change in origin/main. Pull once, then hand control to
# the freshly checked-out copy before reading manifests or invoking install.sh.
if [ "${FUTRX_UPDATE_REEXECED:-0}" != "1" ]; then
    echo "==> Updating repository at $INSTALL_DIR"
    git -C "$INSTALL_DIR" fetch --quiet --tags origin
    git -C "$INSTALL_DIR" reset --hard origin/main
    export FUTRX_UPDATE_REEXECED=1
    exec bash "$INSTALL_DIR/infra/update.sh" "$@"
fi

INFRA_DIR="$INSTALL_DIR/infra"
if [ -z "$HOSTNAME" ] && [ -r "$UNIT" ]; then
    # The installer renders BASE_URL=https://<hostname> into the unit.
    HOSTNAME="$(sed -n 's|^Environment=BASE_URL=https://||p' "$UNIT" | head -1)"
fi
if [ -z "$HOSTNAME" ]; then
    echo "could not detect hostname from $UNIT — pass it explicitly:" >&2
    echo "  sudo bash $0 <hostname>" >&2
    exit 1
fi

if [ "$UPDATE_WORKSPACES" -eq 1 ]; then
    # Rebuild once in install.sh, after the new backend has been built. The
    # workspace upgrader then only needs to recycle containers onto that image.
    FORCE_REBUILD_BASE_IMAGE=1 \
        bash "$INFRA_DIR/install.sh" "$HOSTNAME" --skip-dns-check

    WORKSPACE_ARGS=(--no-rebake)
    if [ "$INCLUDE_BUSY" -eq 1 ]; then
        WORKSPACE_ARGS+=(--include-busy)
    fi
    bash "$INFRA_DIR/upgrade-workspaces.sh" "${WORKSPACE_ARGS[@]}"
else
    bash "$INFRA_DIR/install.sh" "$HOSTNAME" --skip-dns-check
fi

echo
echo "✓ update complete"
