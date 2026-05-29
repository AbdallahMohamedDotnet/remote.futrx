#!/usr/bin/env bash
# code-server: VS Code in the browser, fronted by Caddy at code.${HOSTNAME}.
# Auth is delegated entirely to Caddy's forward_auth gate — code-server runs
# with `auth: none` and binds only to loopback.
#
# Installation: official .deb from coder/code-server releases. We pin a
# version so deploys are deterministic; bump CODE_SERVER_VERSION below to
# upgrade and re-run.
#
# Expects from caller:
#   - log / ok / warn helpers
#   - $INFRA_DIR, $CODE_SERVER_PORT
set -euo pipefail

CODE_SERVER_VERSION="${CODE_SERVER_VERSION:-4.121.0}"

# ───────────────── install (skip if version matches) ─────────────────
CURRENT_VERSION=""
if command -v code-server >/dev/null; then
    CURRENT_VERSION="$(code-server --version 2>/dev/null | head -1 | awk '{print $1}' || true)"
fi

if [ "$CURRENT_VERSION" != "$CODE_SERVER_VERSION" ]; then
    log "Installing code-server $CODE_SERVER_VERSION (current: ${CURRENT_VERSION:-none})"
    # ARM (cax11) is arm64; AMD64 boxes are amd64. dpkg --print-architecture
    # tells us what Debian expects, which matches code-server's release naming.
    ARCH="$(dpkg --print-architecture)"
    URL="https://github.com/coder/code-server/releases/download/v${CODE_SERVER_VERSION}/code-server_${CODE_SERVER_VERSION}_${ARCH}.deb"
    TMP_DEB="$(mktemp --suffix=.deb)"
    trap 'rm -f "$TMP_DEB"' RETURN
    curl -fsSL --retry 3 -o "$TMP_DEB" "$URL"
    apt-get install -y -qq "$TMP_DEB"
fi
ok "code-server $(code-server --version | head -1 | awk '{print $1}')"

# ───────────────── config render ─────────────────
# Always re-render on each run so template changes propagate immediately.
mkdir -p /root/.config/code-server
render_template "${INFRA_DIR}/templates/code-server-config.yaml.tmpl" \
                /root/.config/code-server/config.yaml
chmod 0600 /root/.config/code-server/config.yaml

# ───────────────── systemd ─────────────────
# The Debian package ships a template unit code-server@.service that runs as
# the user passed as the instance name. We use code-server@root (root being
# the install user) which is the same convention as the rest of this box.
if ! systemctl is-enabled --quiet code-server@root.service 2>/dev/null; then
    log "Enabling code-server@root.service"
    systemctl enable code-server@root.service >/dev/null
fi

# Restart picks up config changes. Cheap (~1s) and graceful.
log "Restarting code-server@root.service"
systemctl restart code-server@root.service
sleep 1
if ! systemctl is-active --quiet code-server@root.service; then
    err "code-server failed to start. Recent logs:"
    journalctl -u code-server@root.service -n 20 --no-pager >&2
    exit 1
fi
ok "code-server@root.service is active on 127.0.0.1:${CODE_SERVER_PORT}"
