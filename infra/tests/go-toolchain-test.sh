#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../versions.env
. "$SCRIPT_DIR/../versions.env"
# shellcheck source=../lib/go-toolchain.sh
. "$SCRIPT_DIR/../lib/go-toolchain.sh"

log()  { :; }
warn() { :; }
ok()   { :; }
err()  { :; }

TEST_ROOT="$(command mktemp -d "${TMPDIR:-/tmp}/remote-go-toolchain-test.XXXXXX")"
trap 'command rm -rf "$TEST_ROOT"' EXIT

TEST_ARCH=amd64
CURL_CALLS=0
FAIL_FIRST_DOWNLOAD=0
FAIL_ALL_DOWNLOADS=0
TEST_GO_VERSION="$GO_VERSION"
OLD_GO_VERSION=0.0.1
FAKE_GO_VERSION="$TEST_GO_VERSION"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_eq() {
    local expected="$1" actual="$2" label="$3"
    if [ "$expected" != "$actual" ]; then
        fail "$label: expected '$expected', got '$actual'"
    fi
}

# Production runs on GNU/Linux, where mktemp supports --suffix. This small
# adapter keeps the deterministic fixture test runnable on both Linux and macOS.
mktemp() {
    if [ "${1:-}" = "--suffix=.tgz" ]; then
        command mktemp "${TMPDIR:-/tmp}/remote-go-archive.XXXXXX"
        return
    fi
    command mktemp "$@"
}

dpkg() {
    case "${1:-}" in
        --print-architecture) printf '%s\n' "$TEST_ARCH" ;;
        -s) return 1 ;;
        *) return 1 ;;
    esac
}

write_fake_go_tree() {
    local root="$1" version="$2"
    mkdir -p "$root/bin"
    printf '%s\n' '#!/bin/sh' "echo 'go version go${version} linux/amd64'" > "$root/bin/go"
    printf '%s\n' '#!/bin/sh' 'exit 0' > "$root/bin/gofmt"
    chmod +x "$root/bin/go" "$root/bin/gofmt"
}

# The install function validates and extracts with tar. Tests replace the
# network archive with a tiny deterministic fixture containing fake binaries.
tar() {
    if [ "${1:-}" = "-tzf" ]; then
        return 0
    fi
    local destination=""
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -C) destination="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [ -n "$destination" ] || return 1
    write_fake_go_tree "$destination/go" "$FAKE_GO_VERSION"
}

curl() {
    CURL_CALLS=$((CURL_CALLS + 1))
    local output=""
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -o) output="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    if [ "$FAIL_ALL_DOWNLOADS" -eq 1 ] \
        || { [ "$FAIL_FIRST_DOWNLOAD" -eq 1 ] && [ "$CURL_CALLS" -eq 1 ]; }; then
        return 22
    fi
    [ -n "$output" ] || return 1
    printf 'fixture archive\n' > "$output"
}

use_install_root() {
    local name="$1"
    GO_INSTALL_ROOT="$TEST_ROOT/$name/usr/local"
    export GO_INSTALL_ROOT
    mkdir -p "$GO_INSTALL_ROOT/bin"
    PATH="$GO_INSTALL_ROOT/bin:/usr/bin:/bin"
    export PATH
    hash -r
}

# Debian-to-Go architecture aliases.
assert_eq amd64 "$(go_toolchain_arch amd64)" "amd64 mapping"
assert_eq arm64 "$(go_toolchain_arch arm64)" "arm64 mapping"
assert_eq 386 "$(go_toolchain_arch i386)" "i386 mapping"
assert_eq armv6l "$(go_toolchain_arch armhf)" "armhf mapping"
assert_eq ppc64le "$(go_toolchain_arch ppc64el)" "ppc64el mapping"

# Fresh server: no existing Go installation.
use_install_root fresh
CURL_CALLS=0
FAIL_FIRST_DOWNLOAD=0
FAIL_ALL_DOWNLOADS=0
ensure_go_toolchain "$TEST_GO_VERSION"
assert_eq "$TEST_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "fresh install"
assert_eq 1 "$CURL_CALLS" "fresh download count"

# Idempotent rerun: matching Go must not touch the network.
CURL_CALLS=0
FAIL_ALL_DOWNLOADS=1
ensure_go_toolchain "$TEST_GO_VERSION"
assert_eq 0 "$CURL_CALLS" "idempotent rerun download count"

# Existing/old server: shadow an older distro Go with the exact pin.
use_install_root distro-upgrade
DISTRO_GO_ROOT="$TEST_ROOT/distro-upgrade/usr"
write_fake_go_tree "$DISTRO_GO_ROOT" "$OLD_GO_VERSION"
PATH="$GO_INSTALL_ROOT/bin:$DISTRO_GO_ROOT/bin:/usr/bin:/bin"
export PATH
hash -r
CURL_CALLS=0
FAIL_ALL_DOWNLOADS=0
ensure_go_toolchain "$TEST_GO_VERSION"
assert_eq "$TEST_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "distro Go upgrade"

# Existing pinned/local install: replace it without an unsafe in-place extract.
use_install_root local-upgrade
write_fake_go_tree "$GO_INSTALL_ROOT/go" "$OLD_GO_VERSION"
ln -sf "$GO_INSTALL_ROOT/go/bin/go" "$GO_INSTALL_ROOT/bin/go"
ln -sf "$GO_INSTALL_ROOT/go/bin/gofmt" "$GO_INSTALL_ROOT/bin/gofmt"
hash -r
CURL_CALLS=0
ensure_go_toolchain "$TEST_GO_VERSION"
assert_eq "$TEST_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "local Go upgrade"

# Primary endpoint failure: automatically use the second official endpoint.
use_install_root fallback
CURL_CALLS=0
FAIL_FIRST_DOWNLOAD=1
ensure_go_toolchain "$TEST_GO_VERSION"
assert_eq "$TEST_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "fallback install"
assert_eq 2 "$CURL_CALLS" "fallback download count"

# Total download failure must preserve the old working toolchain.
use_install_root preserve
write_fake_go_tree "$GO_INSTALL_ROOT/go" "$OLD_GO_VERSION"
ln -sf "$GO_INSTALL_ROOT/go/bin/go" "$GO_INSTALL_ROOT/bin/go"
ln -sf "$GO_INSTALL_ROOT/go/bin/gofmt" "$GO_INSTALL_ROOT/bin/gofmt"
hash -r
CURL_CALLS=0
FAIL_FIRST_DOWNLOAD=0
FAIL_ALL_DOWNLOADS=1
if ensure_go_toolchain "$TEST_GO_VERSION"; then
    fail "download failure unexpectedly succeeded"
fi
assert_eq "$OLD_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "preserved old install"

# Unsupported architectures fail before touching an existing installation.
TEST_ARCH=mipsel
if ensure_go_toolchain "$TEST_GO_VERSION"; then
    fail "unsupported architecture unexpectedly succeeded"
fi
assert_eq "$OLD_GO_VERSION" "$(go_toolchain_version "$GO_INSTALL_ROOT/bin/go")" "unsupported architecture preservation"

printf 'PASS: Go toolchain fresh install, upgrade, rerun, fallback, and preservation\n'
