#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib/container-forwarding.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/container-forwarding.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# A stub iptables backed by fake state, so the suite never touches the host
# firewall and can model a machine with or without Docker. Installed rules live
# in a file rather than a variable: callers capture output via command
# substitution, and a subshell's variable writes would not survive.
FORWARD_POLICY="DROP"
HAS_DOCKER_USER=1
INSERT_FAILS=0
RULES_FILE="$(mktemp)"
trap 'rm -f "$RULES_FILE"' EXIT

iptables() {
    case "$1" in
        -S)
            case "${2:-}" in
                FORWARD)
                    printf -- '-P FORWARD %s\n-A FORWARD -j DOCKER-USER\n' "$FORWARD_POLICY"
                    ;;
                DOCKER-USER)
                    [ "$HAS_DOCKER_USER" -eq 1 ] || return 1
                    printf -- '-N DOCKER-USER\n'
                    ;;
            esac
            ;;
        -C)
            grep -qxF "$2 $3 $4" "$RULES_FILE" || return 1
            ;;
        -I)
            [ "$INSERT_FAILS" -eq 0 ] || return 1
            printf '%s\n' "$2 $3 $4" >> "$RULES_FILE"
            ;;
        *)
            return 1
            ;;
    esac
}

reset_state() {
    FORWARD_POLICY="${1:-DROP}"
    HAS_DOCKER_USER="${2:-1}"
    INSERT_FAILS=0
    : > "$RULES_FILE"
}

# Docker present and restricting FORWARD: work is needed, and it belongs in
# the chain Docker preserves for us.
reset_state DROP 1
container_forwarding_needed || fail "expected work to be needed with FORWARD DROP"
[ "$(container_forwarding_chain)" = "DOCKER-USER" ] \
    || fail "expected DOCKER-USER, got '$(container_forwarding_chain)'"

added="$(ensure_container_forwarding lxdbr0)" || fail "ensure returned non-zero"
[ "$(printf '%s\n' "$added" | grep -c .)" -eq 2 ] \
    || fail "expected two rules to be added, got: $added"
printf '%s\n' "$added" | grep -q -- "-i lxdbr0" || fail "inbound rule missing: $added"
printf '%s\n' "$added" | grep -q -- "-o lxdbr0" || fail "outbound rule missing: $added"

# Re-running must be silent: the installer reports only real changes, and a
# re-run must not stack duplicate rules on every update.
added="$(ensure_container_forwarding lxdbr0)" || fail "second ensure returned non-zero"
[ -z "$added" ] || fail "expected no changes on re-run, got: $added"

# No Docker: an unrestricted FORWARD chain needs nothing, and if rules are
# applied anyway they go directly to FORWARD.
reset_state ACCEPT 0
if container_forwarding_needed; then
    fail "expected no work on an unrestricted host without Docker"
fi
[ "$(container_forwarding_chain)" = "FORWARD" ] \
    || fail "expected FORWARD, got '$(container_forwarding_chain)'"

# A host firewall that drops FORWARD without Docker still needs the rules.
reset_state DROP 0
container_forwarding_needed || fail "expected work with FORWARD DROP and no Docker"
added="$(ensure_container_forwarding lxdbr0)" || fail "ensure returned non-zero"
printf '%s\n' "$added" | grep -q "^FORWARD -i lxdbr0$" \
    || fail "expected the rule in FORWARD, got: $added"

# A refused insert must surface, not be silently swallowed — otherwise the
# build proceeds and dies minutes later with an unrelated-looking timeout.
reset_state DROP 1
INSERT_FAILS=1
if ensure_container_forwarding lxdbr0 >/dev/null 2>&1; then
    fail "expected non-zero when the rule could not be added"
fi

# A missing bridge name is a caller bug, not a no-op.
reset_state DROP 1
if ensure_container_forwarding "" >/dev/null 2>&1; then
    fail "expected non-zero for an empty bridge name"
fi

echo "PASS: container-forwarding"
