#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib/health-check.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/health-check.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

REAL_NOW_MS="$(health_check_now_ms)"
case "$REAL_NOW_MS" in
    ''|*[!0-9]*) fail "real clock is not an integer: $REAL_NOW_MS" ;;
esac
[ "$REAL_NOW_MS" -ge 1000000000000 ] || fail "real clock is not milliseconds: $REAL_NOW_MS"

FAKE_NOW_MS=0
REQUEST_COUNT=0
LAST_REQUEST_TIMEOUT=""

health_check_now_ms() {
    printf '%s\n' "$FAKE_NOW_MS"
}

advance_fake_clock() {
    local duration="$1" seconds milliseconds
    seconds="${duration%%.*}"
    milliseconds="${duration#*.}"
    FAKE_NOW_MS="$((FAKE_NOW_MS + 10#$seconds * 1000 + 10#$milliseconds))"
}

health_check_request() {
    local _url="$1" max_time="$2"
    REQUEST_COUNT="$((REQUEST_COUNT + 1))"
    LAST_REQUEST_TIMEOUT="$max_time"
    advance_fake_clock "$max_time"
    return 1
}

health_check_sleep() {
    advance_fake_clock "$1"
}

if wait_for_http_health "http://127.0.0.1:7682/" 30; then
    fail "unhealthy endpoint unexpectedly succeeded"
fi
[ "$FAKE_NOW_MS" -eq 30000 ] || fail "deadline ended at ${FAKE_NOW_MS}ms, want 30000ms"
[ "$REQUEST_COUNT" -eq 8 ] || fail "request count = $REQUEST_COUNT, want 8"
[ "$LAST_REQUEST_TIMEOUT" = "2.000" ] || \
    fail "last request timeout = $LAST_REQUEST_TIMEOUT, want 2.000"

FAKE_NOW_MS=0
REQUEST_COUNT=0
health_check_request() {
    local _url="$1" _max_time="$2"
    REQUEST_COUNT="$((REQUEST_COUNT + 1))"
    [ "$REQUEST_COUNT" -ge 3 ]
}
if ! wait_for_http_health "http://127.0.0.1:7682/" 30; then
    fail "healthy endpoint did not succeed"
fi
[ "$REQUEST_COUNT" -eq 3 ] || fail "success request count = $REQUEST_COUNT, want 3"

echo "health check tests passed"
