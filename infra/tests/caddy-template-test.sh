#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
TEMPLATE="$TESTS_DIR/../templates/Caddyfile.tmpl"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

code_block="$({
    awk '
        /^code[.]\$\{HOSTNAME\} \{/ { in_block = 1 }
        in_block {
            print
            opens += gsub(/\{/, "{")
            closes += gsub(/\}/, "}")
            if (opens > 0 && opens == closes) exit
        }
    ' "$TEMPLATE"
})"

[ -n "$code_block" ] || fail 'code hostname block is missing'
printf '%s\n' "$code_block" | grep -Fq 'tls {' || \
    fail 'code hostname is not assigned an explicit TLS policy'
printf '%s\n' "$code_block" | grep -Fq 'on_demand' || \
    fail 'code hostname does not share the wildcard on-demand TLS policy'

echo 'Caddy template TLS policy tests passed'
