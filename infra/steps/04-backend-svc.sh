#!/usr/bin/env bash
# Backend systemd unit: render, enable, start (or restart on re-run).
# Includes a post-start health check that fails loudly if the binary doesn't
# respond on its loopback port.
#
# Also handles UFW (opens 80/443 if the firewall is active).
#
# Expects from caller:
#   - log / ok / err helpers
#   - $INFRA_DIR, $INSTALL_DIR, $HOSTNAME, $SERVICE_PORT
set -euo pipefail

# ───────────────── systemd unit ─────────────────
log "Rendering /etc/systemd/system/remote.futrx.service"
render_template "${INFRA_DIR}/templates/remote.futrx.service.tmpl" \
                /etc/systemd/system/remote.futrx.service
systemctl daemon-reload

if systemctl is-active --quiet remote.futrx.service; then
    log "Restarting remote.futrx.service"
    systemctl restart remote.futrx.service
else
    log "Starting remote.futrx.service"
    systemctl enable --now remote.futrx.service
fi

# ───────────────── health check ─────────────────
sleep 1
if ! systemctl is-active --quiet remote.futrx.service; then
    err "Service failed to start. Recent logs:"
    journalctl -u remote.futrx.service -n 30 --no-pager >&2
    exit 1
fi

log "Health-checking backend on 127.0.0.1:${SERVICE_PORT}"
HEALTH_OK=0
for _ in 1 2 3 4 5; do
    if curl -fsS --max-time 3 "http://127.0.0.1:${SERVICE_PORT}/" >/dev/null 2>&1; then
        HEALTH_OK=1
        break
    fi
    sleep 1
done
if [ "$HEALTH_OK" -eq 0 ]; then
    err "Backend did not respond on 127.0.0.1:${SERVICE_PORT} within 5s"
    journalctl -u remote.futrx.service -n 30 --no-pager >&2
    exit 1
fi
ok "backend responding"

# ───────────────── UFW ─────────────────
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
    log "Opening UFW for 80 + 443"
    ufw allow 80/tcp  >/dev/null || true
    ufw allow 443/tcp >/dev/null || true
fi
