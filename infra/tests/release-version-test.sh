#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck source=../lib/release-version.sh
. "$TESTS_DIR/../lib/release-version.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TEST_DIR"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_kind() {
    local current="$1" target="$2" want="$3" got
    got="$(release_update_kind "$current" "$target")"
    [ "$got" = "$want" ] || fail "$current -> $target = $got, want $want"
}

assert_kind 0.3.1 0.3.2 application
assert_kind 0.3.1 0.3.1.1 application
assert_kind v0.3.1 v0.3.2 application
assert_kind 0.3.1 0.4.0 infrastructure
assert_kind 0.3.1 0.4.2 infrastructure
assert_kind 0.4.0 0.4.2 application
assert_kind 1.9.5 2.0.0 infrastructure
assert_kind 0.3 0.3.1 infrastructure
assert_kind dev 0.3.2 infrastructure
assert_kind 0.3.1 candidate infrastructure

REPOSITORY="$TEST_DIR/repository"
git init --quiet "$REPOSITORY"
git -C "$REPOSITORY" config user.name Test
git -C "$REPOSITORY" config user.email test@example.com
printf 'release\n' > "$REPOSITORY/source.txt"
git -C "$REPOSITORY" add source.txt
git -C "$REPOSITORY" commit --quiet -m release
release_commit="$(git -C "$REPOSITORY" rev-parse HEAD)"
git -C "$REPOSITORY" tag 1.2.3
git -C "$REPOSITORY" tag -a 1.2.4 -m 1.2.4
git -C "$REPOSITORY" tag 9.9

[ "$(release_build_version "$REPOSITORY" 1.2.3)" = "1.2.3" ] ||
    fail "selected tag was not preserved when two releases share a commit"
[ "$(release_build_version "$REPOSITORY" 1.2.4)" = "1.2.4" ] ||
    fail "selected annotated tag was not preserved"
[ "$(release_build_version "$REPOSITORY" "")" = "1.2.4" ] ||
    fail "exact checkout did not select its highest complete release tag"

printf 'next release\n' >> "$REPOSITORY/source.txt"
git -C "$REPOSITORY" commit --quiet -am next
git -C "$REPOSITORY" tag 1.3.0
[ "$(release_latest_tag "$REPOSITORY")" = "1.3.0" ] ||
    fail "latest release selection accepted an incomplete or lower tag"

printf 'candidate\n' >> "$REPOSITORY/source.txt"
git -C "$REPOSITORY" commit --quiet -am candidate
candidate_sha="$(git -C "$REPOSITORY" rev-parse HEAD)"
[ "$(release_build_version "$REPOSITORY" "$candidate_sha")" = "qa-${candidate_sha:0:12}" ] ||
    fail "immutable QA candidate was not stamped with its qa-prefixed short SHA"
[ "$(release_build_version "$REPOSITORY" "")" = "dev" ] ||
    fail "untagged developer checkout did not receive the dev label"
if release_build_version "$REPOSITORY" 1.3.0 >/dev/null 2>&1; then
    fail "release build version accepted a tag that does not match HEAD"
fi

git -C "$REPOSITORY" reset --hard --quiet "$release_commit"
if release_build_version "$REPOSITORY" "$candidate_sha" >/dev/null 2>&1; then
    fail "candidate build version accepted a SHA that does not match HEAD"
fi

git -C "$REPOSITORY" tag v2.0.0
git -C "$REPOSITORY" tag v10.0.0
git -C "$REPOSITORY" tag 10.0.0
[ "$(release_latest_tag "$REPOSITORY")" = "10.0.0" ] ||
    fail "mixed v-prefixed tags were not sorted by normalized numeric version"

echo "Release version tests passed"
