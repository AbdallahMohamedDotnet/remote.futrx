#!/usr/bin/env bash

# Moves the pre-rename checkout to its canonical path without copying or
# overwriting data. A compatibility symlink keeps the legacy systemd unit
# runnable until the new unit has passed its health check.
migrate_legacy_install_dir() {
    local canonical_dir="$1" legacy_dir="$2"
    local canonical_target legacy_target

    FUTRX_INSTALL_PATH_MIGRATED=0

    if [ -L "$legacy_dir" ]; then
        canonical_target="$(readlink -f -- "$canonical_dir" 2>/dev/null || true)"
        legacy_target="$(readlink -f -- "$legacy_dir" 2>/dev/null || true)"
        if [ -n "$canonical_target" ] && [ "$legacy_target" = "$canonical_target" ]; then
            return 0
        fi
        echo "$legacy_dir is a symlink to an unexpected target; refusing to replace it" >&2
        return 1
    fi

    if [ ! -e "$legacy_dir" ]; then
        return 0
    fi
    if [ ! -d "$legacy_dir/.git" ]; then
        echo "$legacy_dir exists but is not an installed git checkout; refusing to move it" >&2
        return 1
    fi
    if [ -e "$canonical_dir" ]; then
        if [ -d "$canonical_dir" ] && [ -z "$(ls -A "$canonical_dir" 2>/dev/null)" ]; then
            rmdir -- "$canonical_dir"
        else
            echo "both $legacy_dir and $canonical_dir exist; refusing to overwrite either install" >&2
            return 1
        fi
    fi

    echo "==> Migrating installation from $legacy_dir to $canonical_dir"
    if ! mv -- "$legacy_dir" "$canonical_dir"; then
        echo "could not move the legacy installation to $canonical_dir" >&2
        return 1
    fi
    if ! ln -s -- "$canonical_dir" "$legacy_dir"; then
        echo "could not create the compatibility link; rolling the installation back" >&2
        if ! mv -- "$canonical_dir" "$legacy_dir"; then
            echo "rollback failed; installation remains at $canonical_dir" >&2
        fi
        return 1
    fi
    FUTRX_INSTALL_PATH_MIGRATED=1
}

installed_hostname_from_units() {
    local unit hostname
    for unit in "$@"; do
        [ -r "$unit" ] || continue
        hostname="$(sed -n 's|^Environment=BASE_URL=https://||p' "$unit" | head -1)"
        if [ -n "$hostname" ]; then
            printf '%s\n' "$hostname"
            return 0
        fi
    done
    return 1
}

prepare_legacy_service_migration() {
    local legacy_service="$1" legacy_unit_path="$2"

    FUTRX_LEGACY_SERVICE_PRESENT=0
    FUTRX_LEGACY_SERVICE_WAS_ACTIVE=0
    FUTRX_LEGACY_SERVICE_WAS_ENABLED=0

    if systemctl is-active --quiet "$legacy_service"; then
        FUTRX_LEGACY_SERVICE_PRESENT=1
        FUTRX_LEGACY_SERVICE_WAS_ACTIVE=1
    fi
    if systemctl is-enabled --quiet "$legacy_service"; then
        FUTRX_LEGACY_SERVICE_PRESENT=1
        FUTRX_LEGACY_SERVICE_WAS_ENABLED=1
    fi
    if [ -e "$legacy_unit_path" ] || [ -L "$legacy_unit_path" ]; then
        FUTRX_LEGACY_SERVICE_PRESENT=1
    fi
    [ "$FUTRX_LEGACY_SERVICE_PRESENT" -eq 1 ] || return 0

    echo "==> Pausing legacy service $legacy_service"
    if [ "$FUTRX_LEGACY_SERVICE_WAS_ACTIVE" -eq 1 ]; then
        if ! systemctl stop "$legacy_service"; then
            echo "could not stop legacy service $legacy_service" >&2
            return 1
        fi
    fi
    if [ "$FUTRX_LEGACY_SERVICE_WAS_ENABLED" -eq 1 ]; then
        if ! systemctl disable "$legacy_service"; then
            echo "could not disable legacy service $legacy_service; restoring its prior state" >&2
            if ! systemctl enable "$legacy_service"; then
                echo "could not re-enable legacy service $legacy_service" >&2
            fi
            if [ "$FUTRX_LEGACY_SERVICE_WAS_ACTIVE" -eq 1 ]; then
                if ! systemctl start "$legacy_service"; then
                    echo "could not restart legacy service $legacy_service" >&2
                fi
            fi
            return 1
        fi
    fi
}

rollback_legacy_service_migration() {
    local canonical_service="$1" legacy_service="$2"
    local failed=0

    [ "${FUTRX_LEGACY_SERVICE_PRESENT:-0}" -eq 1 ] || return 0
    echo "==> Restoring legacy service $legacy_service" >&2
    if ! systemctl disable --now "$canonical_service" >/dev/null 2>&1; then
        echo "could not stop and disable replacement service $canonical_service" >&2
        failed=1
    fi
    if ! systemctl daemon-reload; then
        echo "could not reload systemd before restoring $legacy_service" >&2
        failed=1
    fi
    if [ "${FUTRX_LEGACY_SERVICE_WAS_ENABLED:-0}" -eq 1 ]; then
        if ! systemctl enable "$legacy_service"; then
            echo "could not re-enable legacy service $legacy_service" >&2
            failed=1
        fi
    fi
    if [ "${FUTRX_LEGACY_SERVICE_WAS_ACTIVE:-0}" -eq 1 ]; then
        if ! systemctl start "$legacy_service"; then
            echo "could not restart legacy service $legacy_service" >&2
            failed=1
        fi
    fi
    [ "$failed" -eq 0 ]
}

complete_legacy_service_migration() {
    local legacy_service="$1" legacy_unit_path="$2"

    [ "${FUTRX_LEGACY_SERVICE_PRESENT:-0}" -eq 1 ] || return 0
    if ! systemctl disable --now "$legacy_service" >/dev/null 2>&1; then
        echo "could not stop and disable legacy service $legacy_service" >&2
        return 1
    fi
    if [ -e "$legacy_unit_path" ] || [ -L "$legacy_unit_path" ]; then
        if ! rm -f -- "$legacy_unit_path"; then
            echo "could not remove legacy unit $legacy_unit_path" >&2
            return 1
        fi
    fi
    if ! systemctl daemon-reload; then
        echo "could not reload systemd after removing $legacy_service" >&2
        return 1
    fi
    echo "==> Removed legacy service $legacy_service"
}
