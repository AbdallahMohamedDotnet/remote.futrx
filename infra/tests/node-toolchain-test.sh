#!/usr/bin/env bash
set -euo pipefail
INFRA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "$INFRA_DIR/versions.env"
. "$INFRA_DIR/lib/node-toolchain.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
TEST_NODE_VERSION=missing
installed="$NODE_MIN_VERSION"
install_calls=0
install_status=0
node() {
    [ "$TEST_NODE_VERSION" != missing ] || return 127
    printf '%s\n' "$TEST_NODE_VERSION"
}
install_nodesource_node() {
    [ "$1" = "$NODE_MAJOR" ] || fail 'wrong NodeSource stream'
    install_calls=$((install_calls + 1))
    TEST_NODE_VERSION="$installed"
    return "$install_status"
}

for initial in missing 22.19.0 22.23.3 24.22.1 22.23.4-rc.1 malformed; do
    TEST_NODE_VERSION="$initial"
    install_calls=0
    ensure_node_toolchain "$NODE_MAJOR" "$NODE_MIN_VERSION" || fail "could not upgrade $initial"
    [ "$install_calls" = 1 ] || fail "$initial was not upgraded"
    require_node_version "$NODE_MAJOR" "$NODE_MIN_VERSION" || fail 'upgrade did not converge'
done

for TEST_NODE_VERSION in "$NODE_MIN_VERSION" 22.23.5; do
    install_calls=0
    ensure_node_toolchain "$NODE_MAJOR" "$NODE_MIN_VERSION" || fail 'healthy Node rejected'
    [ "$install_calls" = 0 ] || fail 'healthy Node was reinstalled'
done

# A successful APT invocation cannot mask repository lag or a shadowing binary.
for installed in 22.23.3 24.22.1 missing; do
    TEST_NODE_VERSION=22.19.0
    if ensure_node_toolchain "$NODE_MAJOR" "$NODE_MIN_VERSION"; then
        fail "accepted inadequate installation $installed"
    fi
done
installed="$NODE_MIN_VERSION"
install_status=1
TEST_NODE_VERSION=22.19.0
if ensure_node_toolchain "$NODE_MAJOR" "$NODE_MIN_VERSION"; then
    fail 'ignored package installer failure'
fi
echo 'Node toolchain tests passed'
