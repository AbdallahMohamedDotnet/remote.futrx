#!/usr/bin/env bash
#
# remote.futrx.dev — main installer / deployer.
# Walks through infra/steps/*.sh in order. Idempotent: safe to re-run on
# every CI deploy as well as the initial bootstrap.
#
# Usage (from a clone):
#   sudo bash infra/install.sh <hostname> [flags]
#
# Usage (fresh box, curl|bash):
#   curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/infra/install.sh \
#     | sudo bash -s -- <hostname> [flags]
#
# Flags:
#   --skip-dns-check                            useful on cloud bootstrap where the public
#                                               A record is set but propagation isn't done.
#   --github-token=ghp_xxx                      private-repo PAT.
#   --google-client-id=...
#   --google-client-secret=...
#
# Environment:
#   GITHUB_TOKEN                                same as --github-token=.
#   CODE_SERVER_VERSION                         override the pinned code-server version.

set -euo pipefail

# ───────────────── self-bootstrap (curl|bash mode) ─────────────────
# When piped from curl, BASH_SOURCE points at /dev/stdin and there are no
# sibling steps/ or templates/. Install git, clone the repo to the canonical
# install dir, and re-exec from there so the rest of this script sees real
# files on disk.
INFRA_DIR_PROBE="$( cd "$( dirname "${BASH_SOURCE[0]:-}" 2>/dev/null || echo . )" >/dev/null 2>&1 && pwd || true )"
if [ -z "$INFRA_DIR_PROBE" ] || [ ! -d "${INFRA_DIR_PROBE}/steps" ]; then
    if [ "$EUID" -ne 0 ]; then
        echo "this installer needs root; rerun with sudo" >&2
        exit 1
    fi
    export DEBIAN_FRONTEND=noninteractive
    if ! command -v git >/dev/null; then
        apt-get update -qq
        apt-get install -y -qq git ca-certificates
    fi

    TARGET="/opt/remote.futrx.dev"

    # Honor --github-token / GITHUB_TOKEN for the bootstrap clone of a private
    # repo. The full arg loop later re-parses everything.
    BOOTSTRAP_TOKEN="${GITHUB_TOKEN:-}"
    for a in "$@"; do
        case "$a" in
            --github-token=*) BOOTSTRAP_TOKEN="${a#*=}" ;;
        esac
    done
    CLONE_URL="https://github.com/Kings-Of-The-Web/remote.futrx.dev.git"
    if [ -n "$BOOTSTRAP_TOKEN" ]; then
        CLONE_URL="https://x-access-token:${BOOTSTRAP_TOKEN}@github.com/Kings-Of-The-Web/remote.futrx.dev.git"
    fi

    if [ ! -d "$TARGET/.git" ]; then
        echo "==> bootstrapping: cloning repo to $TARGET"
        if [ -d "$TARGET" ] && [ -n "$(ls -A "$TARGET" 2>/dev/null)" ]; then
            echo "$TARGET exists and is not empty — refusing to overwrite" >&2
            exit 1
        fi
        mkdir -p "$TARGET"
        git clone --depth=1 "$CLONE_URL" "$TARGET"
        chmod 0600 "$TARGET/.git/config"
    else
        echo "==> repo already at $TARGET, pulling latest"
        git -C "$TARGET" fetch --quiet --tags origin
        git -C "$TARGET" reset --hard origin/main
    fi

    exec bash "$TARGET/infra/install.sh" "$@"
fi

# ───────────────── args ─────────────────
HOSTNAME=""
SKIP_DNS_CHECK=0
GOOGLE_CLIENT_ID=""
GOOGLE_CLIENT_SECRET=""
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
for a in "$@"; do
    case "$a" in
        --skip-dns-check)         SKIP_DNS_CHECK=1 ;;
        --google-client-id=*)     GOOGLE_CLIENT_ID="${a#*=}" ;;
        --google-client-secret=*) GOOGLE_CLIENT_SECRET="${a#*=}" ;;
        --github-token=*)         GITHUB_TOKEN="${a#*=}" ;;
        --*) echo "unknown flag: $a" >&2; exit 1 ;;
        *)   [ -z "$HOSTNAME" ] && HOSTNAME="$a" ;;
    esac
done
if [ -z "$HOSTNAME" ]; then
    read -rp "Public hostname (must already point here in DNS): " HOSTNAME || true
fi
if [ -z "$HOSTNAME" ]; then
    echo "hostname is required (pass as first argument)" >&2
    echo "  example: sudo bash infra/install.sh remote.example.com" >&2
    exit 1
fi
# Refuse an IP literal — Let's Encrypt rejects IP identifiers, and rendering
# the Caddyfile with an IP as the hostname produces a config Caddy still
# loads but can't get a cert for, taking the site down silently. A short
# regex catches both IPv4 and bracketed-IPv6 attempts.
if printf '%s' "$HOSTNAME" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}$|^\[.*\]$'; then
    echo "hostname must be a DNS name, not an IP address (got: $HOSTNAME)" >&2
    echo "  Let's Encrypt cannot issue certs for IPs and your site will lose TLS." >&2
    exit 1
fi
if [ "$EUID" -ne 0 ]; then
    echo "this installer needs root; rerun with sudo" >&2
    exit 1
fi

export HOSTNAME GITHUB_TOKEN GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET

# ───────────────── globals ─────────────────
INFRA_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
INSTALL_DIR="/opt/remote.futrx.dev"
REPO_URL="https://github.com/Kings-Of-The-Web/remote.futrx.dev.git"
SERVICE_PORT="${SERVICE_PORT:-7682}"
CODE_SERVER_PORT="${CODE_SERVER_PORT:-8080}"

# Escape dots in HOSTNAME for Caddy regex (dots match any char in regex; we
# want literal matches).
HOSTNAME_RE="$(printf '%s' "$HOSTNAME" | sed 's/\./\\./g')"

export INFRA_DIR INSTALL_DIR REPO_URL SERVICE_PORT CODE_SERVER_PORT HOSTNAME_RE

# ───────────────── helpers (sourced by steps) ─────────────────
log()  { printf "\n\033[1;36m==> %s\033[0m\n" "$*"; }
warn() { printf "\n\033[1;33m!! %s\033[0m\n" "$*"; }
ok()   { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
err()  { printf "\n\033[1;31m✗ %s\033[0m\n" "$*" >&2; }
export -f log warn ok err

# render_template TEMPLATE_PATH DEST_PATH
# Whitelisted envsubst — only the variables we name get substituted, so
# stray $-prefixed strings in the template (e.g. Caddy's `{re.host.1}`,
# regex `\$` anchors) survive untouched.
render_template() {
    local tmpl="$1" dest="$2"
    envsubst '$HOSTNAME $HOSTNAME_RE $INSTALL_DIR $SERVICE_PORT $CODE_SERVER_PORT $LXD_BRIDGE_IP' \
        < "$tmpl" > "$dest"
}
export -f render_template

# ───────────────── distro guard ─────────────────
if [ ! -r /etc/os-release ]; then
    echo "cannot detect distro — /etc/os-release missing" >&2
    exit 1
fi
# shellcheck source=/dev/null
. /etc/os-release
case "${ID:-}" in
    ubuntu|debian) ;;
    *)
        echo "only Ubuntu/Debian supported (detected: ${ID:-unknown})." >&2
        exit 1
        ;;
esac

# ───────────────── DNS sanity check ─────────────────
# Best-effort verification that the hostname resolves to this server.
# Caddy's ACME challenge fails without correct DNS; we'd rather fail here
# than after spending 30s building.
if [ "$SKIP_DNS_CHECK" -eq 0 ]; then
    log "Checking DNS: $HOSTNAME"
    SERVER_IP=""
    for endpoint in https://api.ipify.org https://ifconfig.me https://ipv4.icanhazip.com; do
        SERVER_IP=$(curl -4 -fsSL --max-time 5 "$endpoint" 2>/dev/null | tr -d '[:space:]' || true)
        [ -n "$SERVER_IP" ] && break
    done
    if [ -z "$SERVER_IP" ]; then
        warn "Could not detect this server's public IPv4 — skipping DNS check."
    else
        HOSTNAME_IPS=$(getent ahostsv4 "$HOSTNAME" 2>/dev/null | awk '{print $1}' | sort -u || true)
        if [ -z "$HOSTNAME_IPS" ]; then
            err "$HOSTNAME does not resolve."
            echo "  Server's public IPv4: $SERVER_IP" >&2
            echo "  Add an A record for $HOSTNAME → $SERVER_IP and wait for propagation." >&2
            echo "  Or re-run with --skip-dns-check (Cloudflare proxy / tunnels / etc)." >&2
            exit 1
        elif ! echo "$HOSTNAME_IPS" | grep -qx "$SERVER_IP"; then
            err "$HOSTNAME resolves to $(echo "$HOSTNAME_IPS" | paste -sd, -), not $SERVER_IP."
            echo "  Update the A record or re-run with --skip-dns-check." >&2
            exit 1
        fi
        ok "$HOSTNAME → $SERVER_IP (matches this server)"
    fi
fi

# ───────────────── run the steps ─────────────────
# shellcheck source=steps/01-host-deps.sh
. "$INFRA_DIR/steps/01-host-deps.sh"
# shellcheck source=steps/02-code-server.sh
. "$INFRA_DIR/steps/02-code-server.sh"
# shellcheck source=steps/03-app.sh
. "$INFRA_DIR/steps/03-app.sh"
# shellcheck source=steps/04-caddy.sh
. "$INFRA_DIR/steps/04-caddy.sh"
# shellcheck source=steps/05-backend-svc.sh
. "$INFRA_DIR/steps/05-backend-svc.sh"
# shellcheck source=steps/06-base-image.sh
. "$INFRA_DIR/steps/06-base-image.sh"

# ───────────────── summary ─────────────────
cat <<EOF

═══════════════════════════════════════════════════════════════
 ✓ Installed at:  $INSTALL_DIR
 ✓ Main UI:       https://$HOSTNAME
 ✓ Code editor:   https://code.$HOSTNAME
 ✓ Dev URLs:      https://<slug>--<port>.dev.$HOSTNAME
 ✓ DB viewers:    lazy per project at https://<slug>--18080.dev.$HOSTNAME
 ✓ Base image:    futrx-remote-dev-base (project containers launch from this)

 ${AUTH_NOTE:-(no auth note)}

 Next:
   1. claude login         # interactive — authenticate the Claude CLI
   2. open Settings and sign in to Codex with ChatGPT if you want Codex
   3. open https://$HOSTNAME (Caddy fetches the cert on first hit, ~10s)

 If you're on a cloud VPS with its own firewall, open 80/443 in the
 provider's console as well as UFW.

 Manage:
   systemctl status   remote.futrx.dev
   systemctl status   code-server@root
   systemctl status   caddy
   journalctl -u      remote.futrx.dev -f

 Re-run this installer any time to pull latest + rebuild + restart.
═══════════════════════════════════════════════════════════════

EOF
