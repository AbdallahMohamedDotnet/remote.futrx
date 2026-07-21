#!/usr/bin/env bash
# remote.futrx.dev — update an already-installed box.
#
# install.sh is idempotent, so "update" is just re-running it: pull latest,
# converge host toolchain + agent CLIs to the pins in infra/versions.env and
# agent-cli-versions.env, rebuild, restart. This wrapper saves you from
# remembering the hostname — it reads it back from the installed systemd
# unit — and skips the DNS pre-check (DNS was already validated at install).
#
# Usage:
#   sudo bash /opt/remote.futrx.dev/infra/update.sh              # hostname auto-detected
#   sudo bash /opt/remote.futrx.dev/infra/update.sh <hostname>   # explicit override
set -euo pipefail

INFRA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
UNIT=/etc/systemd/system/remote.futrx.dev.service

HOSTNAME="${1:-}"
if [ -z "$HOSTNAME" ] && [ -r "$UNIT" ]; then
    # The installer renders BASE_URL=https://<hostname> into the unit.
    HOSTNAME="$(sed -n 's|^Environment=BASE_URL=https://||p' "$UNIT" | head -1)"
fi
if [ -z "$HOSTNAME" ]; then
    echo "could not detect hostname from $UNIT — pass it explicitly:" >&2
    echo "  sudo bash $0 <hostname>" >&2
    exit 1
fi

exec bash "$INFRA_DIR/install.sh" "$HOSTNAME" --skip-dns-check
