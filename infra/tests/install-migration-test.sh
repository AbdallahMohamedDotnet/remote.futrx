#!/usr/bin/env bash
set -euo pipefail

TEST_DIR="$(mktemp -d)"
trap 'command rm -rf -- "$TEST_DIR"' EXIT

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
FAIL_STOP=0
FAIL_DISABLE=0
FAIL_CANONICAL_DISABLE=0
FAIL_ENABLE=0
FAIL_START=0
FAIL_DAEMON_RELOAD=0
FAIL_UNIT_REMOVE=0

rm() {
    local target="${!#}"
    if [ "$FAIL_UNIT_REMOVE" -eq 1 ] && [ "$target" = "$LEGACY_UNIT" ]; then
        return 1
    fi
    command rm "$@"
}

systemctl() {
    local command="$1" service="${!#}"
    printf '%s\n' "$*" >> "$SYSTEMCTL_LOG"
    case "$command" in
        is-active)  [ "$service" = "remote.futrx.dev.service" ] && [ "$LEGACY_ACTIVE" -eq 1 ] ;;
        is-enabled) [ "$service" = "remote.futrx.dev.service" ] && [ "$LEGACY_ENABLED" -eq 1 ] ;;
        stop)
            if [ "$service" = "remote.futrx.dev.service" ]; then
                [ "$FAIL_STOP" -eq 0 ] || return 1
                LEGACY_ACTIVE=0
            fi
            ;;
        disable)
            if [ "$service" = "remote.futrx.dev.service" ]; then
                LEGACY_ENABLED=0
                [ "$2" != "--now" ] || LEGACY_ACTIVE=0
                [ "$FAIL_DISABLE" -eq 0 ] || return 1
            elif [ "$service" = "remote.futrx.service" ]; then
                [ "$FAIL_CANONICAL_DISABLE" -eq 0 ] || return 1
            fi
            ;;
        enable)
            [ "$FAIL_ENABLE" -eq 0 ] || return 1
            [ "$service" != "remote.futrx.dev.service" ] || LEGACY_ENABLED=1
            ;;
        start)
            [ "$FAIL_START" -eq 0 ] || return 1
            [ "$service" != "remote.futrx.dev.service" ] || LEGACY_ACTIVE=1
            ;;
        daemon-reload) [ "$FAIL_DAEMON_RELOAD" -eq 0 ] ;;
        *) fail "unexpected systemctl command: $*" ;;
    esac
}

FAIL_STOP=1
if prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT" 2>/dev/null; then
    fail "legacy service stop failure was ignored"
fi
[ "$LEGACY_ACTIVE" -eq 1 ] || fail "stop failure changed the active state"
[ "$LEGACY_ENABLED" -eq 1 ] || fail "stop failure disabled the legacy service"
FAIL_STOP=0

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
[ "$LEGACY_ACTIVE" -eq 0 ] || fail "legacy service was not stopped"
[ "$LEGACY_ENABLED" -eq 0 ] || fail "legacy service was not disabled"
rollback_legacy_service_migration "remote.futrx.service" "remote.futrx.dev.service"
[ "$LEGACY_ACTIVE" -eq 1 ] || fail "legacy service was not restarted during rollback"
[ "$LEGACY_ENABLED" -eq 1 ] || fail "legacy service was not re-enabled during rollback"

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
FAIL_ENABLE=1
if rollback_legacy_service_migration "remote.futrx.service" "remote.futrx.dev.service" 2>/dev/null; then
    fail "legacy service restore-enable failure was masked by a successful start"
fi
FAIL_ENABLE=0
LEGACY_ACTIVE=1
LEGACY_ENABLED=1

FAIL_DISABLE=1
if prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"; then
    fail "legacy service disable failure was ignored"
fi
[ "$LEGACY_ACTIVE" -eq 1 ] || fail "active service was not restored after disable failure"
[ "$LEGACY_ENABLED" -eq 1 ] || fail "enabled service was not restored after disable failure"
FAIL_DISABLE=0

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
FAIL_CANONICAL_DISABLE=1
if rollback_legacy_service_migration "remote.futrx.service" "remote.futrx.dev.service" 2>/dev/null; then
    fail "replacement service disable failure was ignored during rollback"
fi
FAIL_CANONICAL_DISABLE=0
LEGACY_ACTIVE=1
LEGACY_ENABLED=1

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
FAIL_DAEMON_RELOAD=1
if rollback_legacy_service_migration "remote.futrx.service" "remote.futrx.dev.service" 2>/dev/null; then
    fail "daemon-reload failure was masked by successful legacy service restoration"
fi
FAIL_DAEMON_RELOAD=0
LEGACY_ACTIVE=1
LEGACY_ENABLED=1

prepare_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
FAIL_DISABLE=1
if complete_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT" 2>/dev/null; then
    fail "legacy service cleanup-disable failure was ignored"
fi
[ -e "$LEGACY_UNIT" ] || fail "unit was removed after cleanup-disable failure"
FAIL_DISABLE=0

FAIL_UNIT_REMOVE=1
if complete_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT" 2>/dev/null; then
    fail "legacy unit removal failure was ignored"
fi
[ -e "$LEGACY_UNIT" ] || fail "failed unit removal deleted the unit"
FAIL_UNIT_REMOVE=0

complete_legacy_service_migration "remote.futrx.dev.service" "$LEGACY_UNIT"
[ ! -e "$LEGACY_UNIT" ] || fail "legacy unit file was not removed after success"
[ "$LEGACY_ACTIVE" -eq 0 ] || fail "legacy service remained active after success"
[ "$LEGACY_ENABLED" -eq 0 ] || fail "legacy service remained enabled after success"

echo "install migration tests passed"
