#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib/dns-resolve.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/dns-resolve.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# Every case below stubs curl/getent, so the suite stays hermetic and does
# not depend on the network or on any real hostname continuing to exist.
DOH_BODY=""
DOH_EXIT=0
CURL_CALLS=0

curl() {
    CURL_CALLS=$((CURL_CALLS + 1))
    [ "$DOH_EXIT" -eq 0 ] || return "$DOH_EXIT"
    printf '%s' "$DOH_BODY"
}

GETENT_OUTPUT=""
getent() {
    printf '%s' "$GETENT_OUTPUT"
}

# A normal answer yields the address, and CNAME "data" fields are ignored.
DOH_BODY='{"Status":0,"Answer":[{"name":"a.example","type":5,"data":"target.example."},{"name":"target.example","type":1,"TTL":300,"data":"203.0.113.7"}]}'
got="$(resolve_public_a a.example | paste -sd, -)"
[ "$got" = "203.0.113.7" ] || fail "expected 203.0.113.7, got '$got'"

# Whitespace around the JSON separator must still parse.
DOH_BODY='{"Status": 0, "Answer": [{"data" : "198.51.100.9"}]}'
got="$(resolve_public_a a.example | paste -sd, -)"
[ "$got" = "198.51.100.9" ] || fail "expected 198.51.100.9 from spaced JSON, got '$got'"

# Multiple A records come back sorted and de-duplicated.
DOH_BODY='{"Status":0,"Answer":[{"data":"203.0.113.9"},{"data":"203.0.113.7"},{"data":"203.0.113.9"}]}'
got="$(resolve_public_a a.example | paste -sd, -)"
[ "$got" = "203.0.113.7,203.0.113.9" ] || fail "expected sorted unique pair, got '$got'"

# NXDOMAIN is a definitive answer: no addresses, and no second lookup.
DOH_BODY='{"Status":3,"Question":[{"name":"nope.example","type":1}]}'
CURL_CALLS=0
got="$(resolve_public_a nope.example | paste -sd, -)"
[ -z "$got" ] || fail "expected no addresses for NXDOMAIN, got '$got'"

# The regression this guards: with public resolvers unreachable, the local
# stack's /etc/hosts entry for the machine's own FQDN (127.0.1.1 on Ubuntu)
# must not be mistaken for a public A record.
DOH_EXIT=7
GETENT_OUTPUT='127.0.1.1       srv1781421.hstgr.cloud
127.0.0.1       localhost'
got="$(resolve_public_a srv1781421.hstgr.cloud | paste -sd, -)"
[ -z "$got" ] || fail "loopback from /etc/hosts leaked into the result: '$got'"

# A genuine address from the local stack still counts when DoH is unreachable.
GETENT_OUTPUT='127.0.1.1       box.example
203.0.113.7     box.example'
got="$(resolve_public_a box.example | paste -sd, -)"
[ "$got" = "203.0.113.7" ] || fail "expected fallback to keep 203.0.113.7, got '$got'"

echo "PASS: dns-resolve"
