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

HOSTNAME="${1:-}"
if [ -z "$HOSTNAME" ]; then
    read -rp "Public hostname (must already point here in DNS): " HOSTNAME
fi
if [ -z "$HOSTNAME" ]; then
    echo "hostname is required" >&2
    exit 1
fi

if [ "$EUID" -ne 0 ]; then
    echo "this installer needs root; rerun with sudo" >&2
    exit 1
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

log()  { printf "\n\033[1;36m==> %s\033[0m\n" "$*"; }
warn() { printf "\n\033[1;33m!! %s\033[0m\n" "$*"; }
ok()   { printf "\033[1;32m✓\033[0m %s\n" "$*"; }

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

if [ -d "$INSTALL_DIR/.git" ]; then
    log "Updating repo at $INSTALL_DIR"
    git -C "$INSTALL_DIR" pull --ff-only
else
    log "Cloning repo to $INSTALL_DIR"
    git clone --depth=1 "$REPO_URL" "$INSTALL_DIR"
fi
cd "$INSTALL_DIR"

# ───────────────── build ─────────────────

log "Building frontend (web → public/)"
(
    cd web
    npm install --silent --no-audit --no-fund 2>&1 | tail -3
    npm run build 2>&1 | tail -5
)

log "Building backend (Go → ./remote)"
go build -trimpath -ldflags="-s -w" -o remote .
ok "$(ls -lh remote | awk '{print $5}') binary"

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
systemctl enable --now caddy >/dev/null 2>&1 || true
systemctl reload caddy
ok "Caddy reloaded"

# ───────────────── systemd ─────────────────

log "Installing systemd unit"
cat > /etc/systemd/system/remote.futrx.dev.service <<EOF
[Unit]
Description=remote.futrx.dev — self-hosted Claude Code chat UI
After=network.target

[Service]
ExecStart=$INSTALL_DIR/remote
WorkingDirectory=$INSTALL_DIR
Environment=HOST=127.0.0.1
Environment=PORT=$SERVICE_PORT
Environment=HOME=/root
Environment=DATA_DIR=$INSTALL_DIR/data
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
    warn "Service failed to start — check: journalctl -u remote.futrx.dev.service -n 50"
    exit 1
fi
ok "remote.futrx.dev.service is active"

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

 ⚠  NO AUTH. The URL is open to anyone on the internet.
    Claude has full read/write/exec on this host. Spend will hit
    your Anthropic account. Treat as dev/demo until you put an
    auth provider in front of it.

 Next:
   1. claude login         # interactive — authenticate the Claude CLI
   2. open https://$HOSTNAME (after DNS + ACME settle, ~30s on first run)

 Manage:
   systemctl status   remote.futrx.dev
   systemctl restart  remote.futrx.dev
   journalctl -u      remote.futrx.dev -f
   tail -f $INSTALL_DIR/data/chats/*/events.jsonl   # live event log

 Re-run this installer any time to pull latest + rebuild + restart.
═══════════════════════════════════════════════════════════════

EOF
