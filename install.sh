#!/usr/bin/env bash
#
# remote.futrx.dev installer
# ───────────────────────────
# One-shot setup for a fresh Ubuntu/Debian server. Installs deps (node, go,
# tmux, caddy, claude-code), clones the repo, builds backend + frontend,
# configures Caddy as a TLS reverse proxy, and starts the systemd service.
#
# NO AUTH IS CONFIGURED. The URL is public-facing to anyone on the internet
# until you put an auth provider in front. Treat as dev/demo until then.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/install.sh | sudo bash -s -- remote.example.com
#
# Or from a clone:
#   sudo ./install.sh remote.example.com
#
# Requirements before running:
#   - root (or sudo)
#   - the chosen hostname's DNS A/AAAA record already points to this server
#     (Caddy uses it to obtain a Let's Encrypt cert on first start)
#

set -euo pipefail

# ───────────────── args ─────────────────

HOSTNAME=""
SKIP_DNS_CHECK=0
GOOGLE_CLIENT_ID=""
GOOGLE_CLIENT_SECRET=""
for a in "$@"; do
    case "$a" in
        --skip-dns-check)        SKIP_DNS_CHECK=1 ;;
        --google-client-id=*)    GOOGLE_CLIENT_ID="${a#*=}" ;;
        --google-client-secret=*) GOOGLE_CLIENT_SECRET="${a#*=}" ;;
        --*) echo "unknown flag: $a" >&2; exit 1 ;;
        *)   [ -z "$HOSTNAME" ] && HOSTNAME="$a" ;;
    esac
done
if [ -z "$HOSTNAME" ]; then
    # `|| true` so `set -e` doesn't kill us when read hits EOF (e.g., curl|bash).
    read -rp "Public hostname (must already point here in DNS): " HOSTNAME || true
fi
if [ -z "$HOSTNAME" ]; then
    echo "hostname is required (pass as first argument)" >&2
    echo "  example: sudo bash install.sh remote.example.com" >&2
    exit 1
fi

if [ "$EUID" -ne 0 ]; then
    echo "this installer needs root; rerun with sudo" >&2
    exit 1
fi

log()  { printf "\n\033[1;36m==> %s\033[0m\n" "$*"; }
warn() { printf "\n\033[1;33m!! %s\033[0m\n" "$*"; }
ok()   { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
err()  { printf "\n\033[1;31m✗ %s\033[0m\n" "$*" >&2; }

# ───────────────── DNS sanity check ─────────────────
# Verify $HOSTNAME's A record points to this server BEFORE we install anything.
# Otherwise Caddy's ACME challenge will fail at the end and the user wastes a
# lot of apt/npm time. Use --skip-dns-check for Cloudflare proxy, tunnels, etc.

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
        # `|| true` so getent's NXDOMAIN exit doesn't kill us via set -e + pipefail.
        HOSTNAME_IPS=$(getent ahostsv4 "$HOSTNAME" 2>/dev/null | awk '{print $1}' | sort -u || true)
        if [ -z "$HOSTNAME_IPS" ]; then
            err "$HOSTNAME does not resolve to any IPv4 address."
            cat <<EOF >&2

  This server's public IPv4: $SERVER_IP

  Before re-running the installer, in your DNS provider:
    Add A record:  $HOSTNAME  →  $SERVER_IP
    Wait 1–15 min for propagation. Check with:  dig +short $HOSTNAME

  If you're using Cloudflare proxy, a tunnel, or another reverse-proxy
  setup where the public hostname doesn't point to this box's IP, re-run with:
    $0 $HOSTNAME --skip-dns-check
EOF
            exit 1
        elif ! echo "$HOSTNAME_IPS" | grep -qx "$SERVER_IP"; then
            err "$HOSTNAME points somewhere else, not this server."
            cat <<EOF >&2

  $HOSTNAME currently resolves to: $(echo "$HOSTNAME_IPS" | paste -sd, -)
  This server's public IPv4:       $SERVER_IP

  Caddy needs the DNS record to point here to obtain a Let's Encrypt cert
  via the HTTP-01 challenge.

  Either:
    - Update the A record to $SERVER_IP and wait for propagation
    - Re-run with --skip-dns-check if you're behind Cloudflare proxy / a tunnel:
        $0 $HOSTNAME --skip-dns-check
EOF
            exit 1
        fi
        ok "$HOSTNAME → $SERVER_IP (matches this server)"
    fi
fi

# ───────────────── distro ─────────────────

if [ ! -r /etc/os-release ]; then
    echo "cannot detect distro — /etc/os-release missing" >&2
    exit 1
fi
. /etc/os-release
case "${ID:-}" in
    ubuntu|debian) ;;
    *)
        echo "only Ubuntu/Debian supported in this installer (detected: ${ID:-unknown})." >&2
        echo "for other distros: install go, node 18+, tmux, caddy, claude-code by hand and follow README." >&2
        exit 1
        ;;
esac

INSTALL_DIR="/opt/remote.futrx.dev"
REPO_URL="https://github.com/Kings-Of-The-Web/remote.futrx.dev.git"
SERVICE_PORT=7682

export DEBIAN_FRONTEND=noninteractive

# ───────────────── system deps ─────────────────

log "Updating apt + base deps"
apt-get update -qq
apt-get install -y -qq git curl ca-certificates gnupg tmux build-essential openssl

# Node 20 if missing or too old
NODE_OK=0
if command -v node >/dev/null; then
    NODE_MAJOR="$(node -v | sed 's/^v\([0-9]*\).*/\1/')"
    [ "$NODE_MAJOR" -ge 18 ] && NODE_OK=1
fi
if [ "$NODE_OK" -eq 0 ]; then
    log "Installing Node 20"
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null
    apt-get install -y -qq nodejs
fi
ok "node $(node -v)  npm $(npm -v)"

# Go
if ! command -v go >/dev/null; then
    log "Installing Go"
    apt-get install -y -qq golang-go
fi
ok "$(go version)"

# Before installing Caddy, verify ports 80 + 443 aren't claimed by something
# else (nginx/apache/another caddy). We tolerate Caddy already holding them.
log "Checking ports 80 + 443 are available"
for p in 80 443; do
    if ss -tln 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${p}\$"; then
        # Find what's listening — only allow if it's caddy.
        owner=$(ss -tlnp 2>/dev/null | grep -E "[:.]${p} " | head -1 || true)
        if ! echo "$owner" | grep -q "caddy"; then
            err "Port ${p} is already in use by another process."
            cat <<EOF >&2

  $owner

  Caddy needs ports 80 and 443 exclusively. Stop the other service first:
    sudo ss -tlnp | grep ':$p '       # see who's listening
    sudo systemctl stop <service>     # stop it
    # then re-run this installer
EOF
            exit 1
        fi
    fi
done
ok "ports 80 + 443 are free (or held by Caddy)"

# Caddy — official repo
if ! command -v caddy >/dev/null; then
    log "Installing Caddy"
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
        | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
        > /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -qq
    apt-get install -y -qq caddy
fi
ok "caddy $(caddy version | head -1)"

# Claude Code CLI — npm-installed
if ! command -v claude >/dev/null; then
    log "Installing Claude Code CLI (via npm)"
    npm install -g @anthropic-ai/claude-code --silent 2>&1 | tail -3
fi
ok "claude $(claude --version 2>&1 | head -1)"

# ───────────────── clone / update repo ─────────────────

if [ -d "$INSTALL_DIR" ] && [ ! -d "$INSTALL_DIR/.git" ]; then
    err "$INSTALL_DIR exists but is not a git checkout."
    echo "  Remove it (or move it aside) and re-run." >&2
    exit 1
fi
if [ -d "$INSTALL_DIR/.git" ]; then
    log "Updating repo at $INSTALL_DIR"
    git -C "$INSTALL_DIR" pull --ff-only
else
    log "Cloning repo to $INSTALL_DIR"
    git clone --depth=1 "$REPO_URL" "$INSTALL_DIR"
fi
cd "$INSTALL_DIR"

# ───────────────── build ─────────────────

log "Building frontend (frontend/ → backend/public/)"
(
    cd frontend
    npm install --silent --no-audit --no-fund 2>&1 | tail -3
    npm run build 2>&1 | tail -5
)

log "Building backend (Go → backend/remote)"
(
    cd backend
    go build -trimpath -ldflags="-s -w" -o remote .
)
ok "$(ls -lh backend/remote | awk '{print $5}') binary"

# ───────────────── data dir + auth secrets ─────────────────
# Pre-create data/ with tight perms so secret files we may write below land
# in a directory only root can list.
mkdir -p "$INSTALL_DIR/data"
chmod 0700 "$INSTALL_DIR/data"

if [ -n "$GOOGLE_CLIENT_ID" ] && [ -n "$GOOGLE_CLIENT_SECRET" ]; then
    log "Writing Google OAuth config (data/oauth.json)"
    # JSON-escape the secret in case it ever contains a quote or backslash.
    # client_id is always URL-safe so no escape needed.
    SECRET_ESC=$(printf '%s' "$GOOGLE_CLIENT_SECRET" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))' 2>/dev/null \
                 || printf '%s' "\"$GOOGLE_CLIENT_SECRET\"")
    cat > "$INSTALL_DIR/data/oauth.json" <<EOF
{
  "googleClientId": "$GOOGLE_CLIENT_ID",
  "googleClientSecret": $SECRET_ESC
}
EOF
    chmod 0600 "$INSTALL_DIR/data/oauth.json"
    ok "Google OAuth enabled; first login at https://$HOSTNAME will claim admin"
    AUTH_NOTE="Google OAuth enabled. First Google login becomes admin."
else
    AUTH_NOTE=$(cat <<EON
NO AUTH. To enable Google OAuth later:
  1. In Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client ID (Web app):
       Authorized redirect URI:  https://$HOSTNAME/auth/google/callback
  2. Re-run this installer with:
       --google-client-id=YOURID.apps.googleusercontent.com \\
       --google-client-secret=YOURSECRET
  3. First Google login then claims admin. Everyone else is locked out.
EON
)
fi

# ───────────────── Caddy ─────────────────

log "Writing Caddyfile for $HOSTNAME"
# We REPLACE /etc/caddy/Caddyfile entirely. If you have other sites here,
# move them into a separate file and `import` it.
cat > /etc/caddy/Caddyfile <<EOF
# remote.futrx.dev — managed by install.sh. Edit and re-run to re-apply.
{
    # Email used for Let's Encrypt ACME registration. Leave empty for now;
    # Caddy works without it but a real address is recommended.
    # email you@example.com
}

$HOSTNAME {
    encode zstd gzip
    reverse_proxy 127.0.0.1:$SERVICE_PORT
}
EOF
systemctl enable caddy >/dev/null 2>&1 || true
# `restart` (not reload) on initial setup: surfaces bind-failure errors that
# a reload would silently swallow. Subsequent runs also do a restart, which
# is fine — Caddy reloads instantly and re-uses cached certs.
systemctl restart caddy
ok "Caddy restarted"

# ───────────────── systemd ─────────────────

log "Installing systemd unit"
cat > /etc/systemd/system/remote.futrx.dev.service <<EOF
[Unit]
Description=remote.futrx.dev — self-hosted Claude Code chat UI
After=network.target

[Service]
ExecStart=$INSTALL_DIR/backend/remote
WorkingDirectory=$INSTALL_DIR
Environment=HOST=127.0.0.1
Environment=PORT=$SERVICE_PORT
Environment=HOME=/root
Environment=DATA_DIR=$INSTALL_DIR/data
Environment=BASE_URL=https://$HOSTNAME
# tmux server inherits this cgroup; KillMode=process spares it on restart.
KillMode=process
Restart=always
RestartSec=2
User=root

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now remote.futrx.dev.service
sleep 1
if ! systemctl is-active --quiet remote.futrx.dev.service; then
    err "Service failed to start. Recent logs:"
    journalctl -u remote.futrx.dev.service -n 30 --no-pager >&2
    exit 1
fi
ok "remote.futrx.dev.service is active"

# Verify the binary actually responds, not just that systemd thinks it's up.
log "Health-checking backend on 127.0.0.1:$SERVICE_PORT"
HEALTH_OK=0
for _ in 1 2 3 4 5; do
    if curl -fsS --max-time 3 "http://127.0.0.1:$SERVICE_PORT/" >/dev/null 2>&1; then
        HEALTH_OK=1
        break
    fi
    sleep 1
done
if [ "$HEALTH_OK" -eq 0 ]; then
    err "Backend did not respond on 127.0.0.1:$SERVICE_PORT within 5s."
    journalctl -u remote.futrx.dev.service -n 30 --no-pager >&2
    exit 1
fi
ok "backend responding"

# ───────────────── firewall ─────────────────

if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
    log "Opening UFW for 80 + 443"
    ufw allow 80/tcp  >/dev/null || true
    ufw allow 443/tcp >/dev/null || true
fi

# ───────────────── summary ─────────────────

cat <<EOF

═══════════════════════════════════════════════════════════════
 ✓ Installed at: $INSTALL_DIR
 ✓ Reverse-proxied at: https://$HOSTNAME

 $AUTH_NOTE

 Next:
   1. claude login         # interactive — authenticate the Claude CLI
   2. open https://$HOSTNAME (Caddy fetches the cert on first hit, ~10s)

 If you're on a cloud VPS with its own firewall (Hetzner Cloud, AWS, GCP,
 DigitalOcean), ALSO open 80 and 443 in the provider's console — UFW only
 manages the OS firewall.

 Manage:
   systemctl status   remote.futrx.dev
   systemctl restart  remote.futrx.dev
   journalctl -u      remote.futrx.dev -f
   tail -f $INSTALL_DIR/data/chats/*/events.jsonl   # live event log

 Re-run this installer any time to pull latest + rebuild + restart.
═══════════════════════════════════════════════════════════════

EOF
