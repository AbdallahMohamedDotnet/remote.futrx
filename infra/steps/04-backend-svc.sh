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
#   - $REMOTE_ENV_FILE, $REMOTE_CLI_PATH
set -euo pipefail

SERVICE_NAME="remote.futrx.service"
SERVICE_UNIT_PATH="/etc/systemd/system/$SERVICE_NAME"
LEGACY_SERVICE_NAME="remote.futrx.dev.service"
LEGACY_SERVICE_UNIT_PATH="/etc/systemd/system/$LEGACY_SERVICE_NAME"
HOST_CLI_PROFILE_PATH="/etc/profile.d/remote-futrx-host-clis.sh"

# shellcheck source=../lib/install-migration.sh
. "$INFRA_DIR/lib/install-migration.sh"
# shellcheck source=../lib/health-check.sh
. "$INFRA_DIR/lib/health-check.sh"

# ───────────────── canonical config + CLI launcher ─────────────────
# Both the systemd unit below and an operator's interactive shell invoke
# $REMOTE_CLI_PATH, which reads $REMOTE_ENV_FILE for BASE_URL/DATA_DIR/
# INSTALL_DIR. Rendering both here, before the unit, keeps them the single
# source of truth instead of duplicating install-specific values into the
# unit file.
log "Rendering $REMOTE_ENV_FILE"
render_template "${INFRA_DIR}/templates/remote.futrx.env.tmpl" \
                "$REMOTE_ENV_FILE"
chown root:root "$REMOTE_ENV_FILE"
chmod 0644 "$REMOTE_ENV_FILE"

# ───────────────── operator group for setup-token ─────────────────
# `remote setup-token` needs to read local-admin.json/users.json and write
# setup-token.json without sudo. Membership in this group is how the
# install-time operator gets that without making those files world- or
# every-local-user-readable; every other file in DATA_DIR (OAuth secret,
# agent API keys, session key) stays root-only 0600 — see
# backend/internal/stores/fileauth/store.go's groupReadableAuthFiles.
DATA_DIR="$INSTALL_DIR/data"
REMOTE_GROUP="remote"
log "Ensuring group $REMOTE_GROUP and operator membership"
if ! getent group "$REMOTE_GROUP" >/dev/null; then
    groupadd --system "$REMOTE_GROUP"
fi
OPERATOR_USER="${SUDO_USER:-}"
if [ -n "$OPERATOR_USER" ] && [ "$OPERATOR_USER" != "root" ] && id "$OPERATOR_USER" >/dev/null 2>&1; then
    usermod -aG "$REMOTE_GROUP" "$OPERATOR_USER"
    ok "$OPERATOR_USER can run 'remote setup-token' without sudo (after their next login)"
else
    warn "Could not determine a non-root operator to add to '$REMOTE_GROUP'; add one manually: sudo usermod -aG $REMOTE_GROUP <you>"
fi

# setgid so files the backend (running as root) creates under DATA_DIR
# inherit group $REMOTE_GROUP instead of root's primary group.
mkdir -p "$DATA_DIR"
chgrp "$REMOTE_GROUP" "$DATA_DIR"
chmod 2750 "$DATA_DIR"
for f in local-admin.json setup-token.json users.json; do
    if [ -f "$DATA_DIR/$f" ]; then
        chgrp "$REMOTE_GROUP" "$DATA_DIR/$f"
        chmod 0640 "$DATA_DIR/$f"
    fi
done

if [ -e "$REMOTE_CLI_PATH" ] && [ -d "$REMOTE_CLI_PATH" ]; then
    err "$REMOTE_CLI_PATH exists and is a directory; refusing to replace it"
    exit 1
fi
log "Installing $REMOTE_CLI_PATH"
REMOTE_CLI_TMP="$(mktemp)"
cp "${INFRA_DIR}/templates/remote-cli.sh.tmpl" "$REMOTE_CLI_TMP"
bash -n "$REMOTE_CLI_TMP"
# install(1) writes through an existing symlink rather than replacing it, so
# a stale link left by a manual repair attempt could get followed to an
# unrelated file. Removing first guarantees $REMOTE_CLI_PATH always ends up a
# plain, root-owned, 0755 regular file.
rm -f -- "$REMOTE_CLI_PATH"
install -o root -g root -m 0755 "$REMOTE_CLI_TMP" "$REMOTE_CLI_PATH"
rm -f -- "$REMOTE_CLI_TMP"

# ───────────────── systemd unit ─────────────────
log "Rendering $HOST_CLI_PROFILE_PATH"
render_template "${INFRA_DIR}/templates/remote-futrx-host-clis.sh.tmpl" \
                "$HOST_CLI_PROFILE_PATH"
chmod 0644 "$HOST_CLI_PROFILE_PATH"

log "Rendering $SERVICE_UNIT_PATH"
render_template "${INFRA_DIR}/templates/remote.futrx.service.tmpl" \
                "$SERVICE_UNIT_PATH"
systemctl daemon-reload

if ! prepare_legacy_service_migration "$LEGACY_SERVICE_NAME" "$LEGACY_SERVICE_UNIT_PATH"; then
    err "Could not pause $LEGACY_SERVICE_NAME; the existing service was left in place."
    exit 1
fi

if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "Restarting $SERVICE_NAME"
    SERVICE_ACTION="restart"
else
    log "Starting $SERVICE_NAME"
    SERVICE_ACTION="enable"
fi
if { [ "$SERVICE_ACTION" = "restart" ] && ! systemctl restart "$SERVICE_NAME"; } || \
   { [ "$SERVICE_ACTION" = "enable" ] && ! systemctl enable --now "$SERVICE_NAME"; }; then
    err "$SERVICE_NAME failed to $SERVICE_ACTION."
    if ! rollback_legacy_service_migration "$SERVICE_NAME" "$LEGACY_SERVICE_NAME"; then
        err "Automatic rollback to $LEGACY_SERVICE_NAME also failed."
    fi
    journalctl -u "$SERVICE_NAME" -n 30 --no-pager >&2 || true
    exit 1
fi

# ───────────────── health check ─────────────────
if ! systemctl is-active --quiet "$SERVICE_NAME"; then
    err "Service failed to start. Recent logs:"
    if ! rollback_legacy_service_migration "$SERVICE_NAME" "$LEGACY_SERVICE_NAME"; then
        err "Automatic rollback to $LEGACY_SERVICE_NAME also failed."
    fi
    journalctl -u "$SERVICE_NAME" -n 30 --no-pager >&2 || true
    exit 1
fi

log "Health-checking backend on 127.0.0.1:${SERVICE_PORT}"
# Cold start binds the port only after converging container resource envelopes
# (~13s with 16 projects), so allow a real 30-second wall-clock deadline.
if ! wait_for_http_health "http://127.0.0.1:${SERVICE_PORT}/" 30; then
    err "Backend did not respond on 127.0.0.1:${SERVICE_PORT} within 30s"
    if ! rollback_legacy_service_migration "$SERVICE_NAME" "$LEGACY_SERVICE_NAME"; then
        err "Automatic rollback to $LEGACY_SERVICE_NAME also failed."
    fi
    journalctl -u "$SERVICE_NAME" -n 30 --no-pager >&2 || true
    exit 1
fi
ok "backend responding"

if ! complete_legacy_service_migration "$LEGACY_SERVICE_NAME" "$LEGACY_SERVICE_UNIT_PATH"; then
    err "Backend is healthy, but cleanup of $LEGACY_SERVICE_NAME failed."
    exit 1
fi

# ───────────────── UFW ─────────────────
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
    log "Opening UFW for 80 + 443"
    ufw allow 80/tcp  >/dev/null || true
    ufw allow 443/tcp >/dev/null || true
fi
