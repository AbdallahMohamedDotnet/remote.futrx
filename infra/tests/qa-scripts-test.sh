#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
INSTALL_SCRIPT="$TESTS_DIR/../qa/install.sh"
UPDATE_SCRIPT="$TESTS_DIR/../qa/update.sh"
COMMON_SCRIPT="$TESTS_DIR/../qa/common.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TEST_DIR"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

for script in "$COMMON_SCRIPT" "$INSTALL_SCRIPT" "$UPDATE_SCRIPT"; do
    bash -n "$script"
done

bash "$INSTALL_SCRIPT" --help | grep -q 'public curl|bash command' || \
    fail "install help does not identify the public installer contract"
bash "$UPDATE_SCRIPT" --help | grep -q 'existing QA installation' || \
    fail "update help does not identify the existing-installation contract"

if QA_ENV_FILE="$TEST_DIR/missing.env" bash "$INSTALL_SCRIPT" >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "install.sh accepted missing QA configuration"
fi
grep -q 'missing .*missing.env' "$TEST_DIR/err" || \
    fail "install.sh gave an unclear missing-config error"

if bash "$INSTALL_SCRIPT" main >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "install.sh accepted a Git ref even though the public installer has no ref argument"
fi
grep -q '^Usage: bash infra/qa/install.sh$' "$TEST_DIR/err" || \
    fail "install.sh gave unclear usage for an unexpected argument"

if QA_ENV_FILE="$TEST_DIR/missing.env" bash "$UPDATE_SCRIPT" main >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "update.sh accepted missing QA configuration"
fi
grep -q 'missing .*missing.env' "$TEST_DIR/err" || \
    fail "update.sh gave an unclear missing-config error"

if QA_ENV_FILE="$TEST_DIR/missing.env" bash "$UPDATE_SCRIPT" 'bad ref' >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "update.sh accepted an unsafe Git ref"
fi
grep -q 'unsupported characters' "$TEST_DIR/err" || \
    fail "update.sh gave an unclear unsafe-ref error"

grep -Fq 'curl -fsSL "$install_url" | sudo bash -s -- "$public_host"' "$INSTALL_SCRIPT" || \
    fail "install.sh does not use the documented public curl pipeline"
if grep -Eq 'apt-get|git clone' "$INSTALL_SCRIPT"; then
    fail "install.sh bootstraps dependencies or clones the repository itself"
fi

echo "QA install/update script tests passed"
