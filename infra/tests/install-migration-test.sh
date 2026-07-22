#!/usr/bin/env bash
set -euo pipefail

TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TEST_DIR"' EXIT

# shellcheck source=../lib/install-migration.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/install-migration.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

CANONICAL_DIR="$TEST_DIR/remote.futrx"
LEGACY_DIR="$TEST_DIR/remote.futrx.dev"
mkdir -p "$LEGACY_DIR/.git" "$LEGACY_DIR/data"
printf 'preserved\n' > "$LEGACY_DIR/data/state"

migrate_legacy_install_dir "$CANONICAL_DIR" "$LEGACY_DIR"
[ "$FUTRX_INSTALL_PATH_MIGRATED" -eq 1 ] || fail "migration was not reported to the caller"
[ -d "$CANONICAL_DIR/.git" ] || fail "canonical checkout was not created"
[ "$(cat "$CANONICAL_DIR/data/state")" = "preserved" ] || fail "data was not preserved"
[ -L "$LEGACY_DIR" ] || fail "legacy compatibility link was not created"
[ "$(readlink -f "$LEGACY_DIR")" = "$(readlink -f "$CANONICAL_DIR")" ] || \
    fail "legacy compatibility link points at the wrong path"
migrate_legacy_install_dir "$CANONICAL_DIR" "$LEGACY_DIR"

mkdir -p "$TEST_DIR/conflict-new/.git" "$TEST_DIR/conflict-old/.git"
if migrate_legacy_install_dir "$TEST_DIR/conflict-new" "$TEST_DIR/conflict-old" 2>/dev/null; then
    fail "conflicting installations were accepted"
fi

CANONICAL_UNIT="$TEST_DIR/remote.futrx.service"
LEGACY_UNIT="$TEST_DIR/remote.futrx.dev.service"
printf 'Environment=BASE_URL=https://new.example.com\n' > "$CANONICAL_UNIT"
printf 'Environment=BASE_URL=https://old.example.com\n' > "$LEGACY_UNIT"
[ "$(installed_hostname_from_units "$CANONICAL_UNIT" "$LEGACY_UNIT")" = "new.example.com" ] || \
    fail "canonical unit hostname was not preferred"
rm -f -- "$CANONICAL_UNIT"
[ "$(installed_hostname_from_units "$CANONICAL_UNIT" "$LEGACY_UNIT")" = "old.example.com" ] || \
    fail "legacy unit hostname was not detected"

SYSTEMCTL_LOG="$TEST_DIR/systemctl.log"
LEGACY_ACTIVE=1
LEGACY_ENABLED=1
FAIL_DISABLE=0
systemctl() {
    local command="$1" service="${!#}"
    printf '%s\n' "$*" >> "$SYSTEMCTL_LOG"
    case "$command" in
        is-active)  [ "$service" = "remote.futrx.dev.service" ] && [ "$LEGACY_ACTIVE" -eq 1 ] ;;
        is-enabled) [ "$service" = "remote.futrx.dev.service" ] && [ "$LEGACY_ENABLED" -eq 1 ] ;;
        stop)        [ "$service" != "remote.futrx.dev.service" ] || LEGACY_ACTIVE=0 ;;
        disable)
            if [ "$service" = "remote.futrx.dev.service" ]; then
                LEGACY_ENABLED=0
                [ "$2" != "--now" ] || LEGACY_ACTIVE=0
                [ "$FAIL_DISABLE" -eq 0 ] || return 1
            fi
            ;;
        enable)      [ "$service" != "remote.futrx.dev.service" ] || LEGACY_ENABLED=1 ;;
        start)       [ "$service" != "remote.futrx.dev.service" ] || LEGACY_ACTIVE=1 ;;
        daemon-reload) ;;
        *) fail "unexpected systemctl command: $*" ;;
    esac
}

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
[ "$LEGACY_ACTIVE" -eq 0 ] || fail "legacy service was not stopped"
[ "$LEGACY_ENABLED" -eq 0 ] || fail "legacy service was not disabled"
rollback_legacy_service_migration "remote.futrx.service" "remote.futrx.dev.service"
[ "$LEGACY_ACTIVE" -eq 1 ] || fail "legacy service was not restarted during rollback"
[ "$LEGACY_ENABLED" -eq 1 ] || fail "legacy service was not re-enabled during rollback"

FAIL_DISABLE=1
if prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"; then
    fail "legacy service disable failure was ignored"
fi
[ "$LEGACY_ACTIVE" -eq 1 ] || fail "active service was not restored after disable failure"
[ "$LEGACY_ENABLED" -eq 1 ] || fail "enabled service was not restored after disable failure"
FAIL_DISABLE=0

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
complete_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
[ ! -e "$LEGACY_UNIT" ] || fail "legacy unit file was not removed after success"
[ "$LEGACY_ACTIVE" -eq 0 ] || fail "legacy service remained active after success"
[ "$LEGACY_ENABLED" -eq 0 ] || fail "legacy service remained enabled after success"

echo "install migration tests passed"
