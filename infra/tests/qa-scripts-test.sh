#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
INSTALL_SCRIPT="$TESTS_DIR/../qa/install.sh"
UPDATE_SCRIPT="$TESTS_DIR/../qa/update.sh"
COMMON_SCRIPT="$TESTS_DIR/../qa/common.sh"
CORE_INSTALL_SCRIPT="$TESTS_DIR/../install.sh"
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

if QA_ENV_FILE="$TEST_DIR/missing.env" bash "$INSTALL_SCRIPT" main >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "install.sh accepted missing QA configuration for a candidate install"
fi
grep -q 'missing .*missing.env' "$TEST_DIR/err" || \
    fail "install.sh gave an unclear candidate missing-config error"

if QA_ENV_FILE="$TEST_DIR/missing.env" bash "$INSTALL_SCRIPT" 'bad ref' >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "install.sh accepted an unsafe Git ref"
fi
grep -q 'unsupported characters' "$TEST_DIR/err" || \
    fail "install.sh gave an unclear unsafe-ref error"

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

if bash "$CORE_INSTALL_SCRIPT" test.example.com --ref=main >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "core install.sh accepted a movable ref"
fi
grep -q '^--ref must be a full 40-character commit SHA$' "$TEST_DIR/err" || \
    fail "core install.sh gave an unclear immutable-ref error"

if FUTRX_INSTALL_DIR="$TEST_DIR/bootstrap-install" \
    bash -s -- test.example.com --ref=main <"$CORE_INSTALL_SCRIPT" \
    >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "curl-mode core install.sh accepted a movable ref"
fi
grep -q '^--ref must be a full 40-character commit SHA$' "$TEST_DIR/err" || \
    fail "curl-mode core install.sh gave an unclear immutable-ref error"

grep -Fq 'curl -fsSL "$install_url" | sudo bash -s -- "$public_host"' "$INSTALL_SCRIPT" || \
    fail "install.sh does not use the documented public curl pipeline"
grep -Fq 'raw.githubusercontent.com/futrx-com/remote.futrx/${CANDIDATE_SHA}/infra/install.sh' "$INSTALL_SCRIPT" || \
    fail "install.sh does not download the installer from the immutable candidate commit"
grep -Fq '"--ref=$candidate_sha"' "$INSTALL_SCRIPT" || \
    fail "install.sh does not pin the bootstrap clone to the candidate commit"
if grep -Eq 'apt-get|git clone' "$INSTALL_SCRIPT"; then
    fail "install.sh bootstraps dependencies or clones the repository itself"
fi

echo "QA install/update script tests passed"
