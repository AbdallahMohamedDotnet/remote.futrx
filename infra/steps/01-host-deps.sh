#!/usr/bin/env bash
# Host-level system dependencies: apt base, Node 20, Go, Caddy, agent CLIs, LXD.
# Idempotent — re-runs are fast no-ops when everything's already installed.
#
# Expects from caller:
#   - log / ok / warn / err helpers
#   - $INFRA_DIR (path to infra/ in the cloned repo)
#   - $HOSTNAME (for diagnostic messages only)
#   - $SKIP_DNS_CHECK (0 / 1)
#
# Sets in environment for later steps:
#   - $LXD_BRIDGE_IP (for the resolved drop-in)
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

# ───────────────── base apt deps ─────────────────
log "apt update + base packages"
apt-get update -qq
apt-get install -y -qq git curl ca-certificates gnupg tmux build-essential openssl gettext-base

# ───────────────── Node 20 ─────────────────
NODE_OK=0
if command -v node >/dev/null; then
    NODE_MAJOR="$(node -v | sed 's/^v\([0-9]*\).*/\1/')"
    [ "$NODE_MAJOR" -ge 18 ] && NODE_OK=1
fi
if [ "$NODE_OK" -eq 0 ]; then
    log "Installing Node 20 from NodeSource"
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null
    apt-get install -y -qq nodejs
fi
ok "node $(node -v)  npm $(npm -v)"

# ───────────────── Go ─────────────────
if ! command -v go >/dev/null; then
    log "Installing Go (distro package)"
    apt-get install -y -qq golang-go
fi
ok "$(go version)"

# ───────────────── ports 80/443 free? ─────────────────
log "Checking ports 80 + 443 are free (or held by Caddy)"
for p in 80 443; do
    if ss -tln 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${p}\$"; then
        owner=$(ss -tlnp 2>/dev/null | grep -E "[:.]${p} " | head -1 || true)
        if ! echo "$owner" | grep -q "caddy"; then
            err "Port ${p} is already in use by another process."
            cat <<EOF >&2

  $owner

  Caddy needs ports 80 and 443 exclusively. Stop the other service first:
    sudo ss -tlnp | grep ':$p '
    sudo systemctl stop <service>
  Then re-run the installer.
EOF
            exit 1
        fi
    fi
done
ok "ports 80 + 443 OK"

# ───────────────── Caddy ─────────────────
if ! command -v caddy >/dev/null; then
    log "Installing Caddy (Cloudsmith repo)"
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
        | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
        > /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -qq
    apt-get install -y -qq caddy
fi
ok "$(caddy version | head -1)"

# ───────────────── agent CLIs (host-side auth/provisioning) ─────────────────
# The same manifest is embedded by the Go container manager. Re-running the
# installer upgrades stale host binaries instead of only checking existence.
AGENT_CLI_VERSIONS_FILE="$INFRA_DIR/../backend/internal/manager/containers/agent-cli-versions.env"
if [ ! -r "$AGENT_CLI_VERSIONS_FILE" ]; then
    err "missing agent CLI version manifest: $AGENT_CLI_VERSIONS_FILE"
    exit 1
fi
# shellcheck source=/dev/null
. "$AGENT_CLI_VERSIONS_FILE"

agent_cli_version() {
    "$1" --version 2>&1 \
        | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?' \
        | head -1 || true
}

ensure_agent_cli() {
    local label="$1" binary="$2" package="$3" expected="$4" current=""
    if command -v "$binary" >/dev/null; then
        current="$(agent_cli_version "$binary")"
    fi
    if [ "$current" != "$expected" ]; then
        log "Installing $label $expected (was ${current:-missing})"
        npm install -g "${package}@${expected}" --silent 2>&1 | tail -3
    fi
    ok "$label $("$binary" --version 2>&1 | head -1)"
}

ensure_agent_cli "Claude Code" claude @anthropic-ai/claude-code "$CLAUDE_CODE_VERSION"
ensure_agent_cli "Codex" codex @openai/codex "$CODEX_CLI_VERSION"

# ───────────────── LXD (one container per project) ─────────────────
if ! command -v lxc >/dev/null; then
    log "Installing LXD via snap"
    if ! command -v snap >/dev/null; then
        apt-get install -y -qq snapd
        systemctl enable --now snapd.socket
        for _ in 1 2 3 4 5; do snap wait system seed.loaded && break; sleep 1; done
    fi
    snap install lxd
    export PATH="/snap/bin:$PATH"
fi

# Initialize storage + bridge on fresh installs. `lxc network show lxdbr0`
# is our "initialized" probe.
if ! lxc network show lxdbr0 >/dev/null 2>&1; then
    log "lxd init --auto"
    lxd init --auto
fi
ok "lxd $(lxc version --format=csv 2>/dev/null | tr ',' ' ' | awk '{print $1}' || echo ok)"

# Detect the bridge IP so the resolved drop-in can forward *.lxd queries.
LXD_BRIDGE_IP=$(lxc network get lxdbr0 ipv4.address 2>/dev/null | sed 's|/.*||')
if [ -z "$LXD_BRIDGE_IP" ]; then
    warn "lxdbr0 bridge IP not detectable — *.dev.${HOSTNAME} routing will fail."
else
    export LXD_BRIDGE_IP
fi

# ───────────────── systemd-resolved: forward *.lxd to the bridge ─────────────────
if [ -n "${LXD_BRIDGE_IP:-}" ] && systemctl is-active --quiet systemd-resolved; then
    log "systemd-resolved drop-in for *.lxd → ${LXD_BRIDGE_IP}"
    mkdir -p /etc/systemd/resolved.conf.d
    render_template "${INFRA_DIR}/templates/lxd-resolved.conf.tmpl" \
                    /etc/systemd/resolved.conf.d/lxd.conf
    systemctl restart systemd-resolved
fi
