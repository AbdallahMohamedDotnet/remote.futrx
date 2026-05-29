#!/usr/bin/env bash
#
# remote.futrx.dev — installer shim.
#
# The actual installer lives in infra/ as a set of modular bash files.
# This shim exists to keep the curl|bash flow working when there's no
# local clone yet:
#
#   curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/install.sh \
#     | sudo bash -s -- remote.example.com
#
# When run from a local clone of the repo, it just execs infra/install.sh.
#
# Read infra/README.md for the layout and per-step responsibilities.

set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if [ -d "$DIR/infra" ] && [ -f "$DIR/infra/install.sh" ]; then
    exec bash "$DIR/infra/install.sh" "$@"
fi

# curl|bash mode — fetch the repo, then re-exec the real installer.
if [ "$EUID" -ne 0 ]; then
    echo "this installer needs root; rerun with sudo" >&2
    exit 1
fi

command -v git >/dev/null || {
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq git ca-certificates
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Honor GITHUB_TOKEN / --github-token for private-repo clones in the curl|bash
# path. The flag parsing here is just lightweight — the real installer
# re-parses everything.
TOKEN="${GITHUB_TOKEN:-}"
for a in "$@"; do
    case "$a" in
        --github-token=*) TOKEN="${a#*=}" ;;
    esac
done
CLONE_URL="https://github.com/Kings-Of-The-Web/remote.futrx.dev.git"
if [ -n "$TOKEN" ]; then
    CLONE_URL="https://x-access-token:${TOKEN}@github.com/Kings-Of-The-Web/remote.futrx.dev.git"
fi

echo "==> Fetching repo to $TMP (curl|bash mode)"
git clone --depth=1 "$CLONE_URL" "$TMP" >/dev/null

exec bash "$TMP/infra/install.sh" "$@"
