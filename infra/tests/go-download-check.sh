#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../versions.env
. "$SCRIPT_DIR/../versions.env"
# shellcheck source=../lib/go-toolchain.sh
. "$SCRIPT_DIR/../lib/go-toolchain.sh"

if command -v dpkg >/dev/null 2>&1; then
    DEB_ARCH="$(dpkg --print-architecture)"
else
    case "$(uname -m)" in
        x86_64)       DEB_ARCH=amd64 ;;
        arm64|aarch64) DEB_ARCH=arm64 ;;
        *)
            printf 'Unsupported local test architecture: %s\n' "$(uname -m)" >&2
            exit 1
            ;;
    esac
fi
if ! GO_ARCH="$(go_toolchain_arch "$DEB_ARCH")"; then
    printf 'Unsupported CI architecture: %s\n' "$DEB_ARCH" >&2
    exit 1
fi

FILENAME="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
URLS=(
    "https://dl.google.com/go/${FILENAME}"
    "https://go.dev/dl/${FILENAME}"
)

for url in "${URLS[@]}"; do
    if curl --fail --silent --show-error --location --head \
        --connect-timeout 20 --retry 3 --retry-delay 2 "$url" >/dev/null; then
        printf 'PASS: Go %s is available for %s at %s\n' "$GO_VERSION" "$GO_ARCH" "$url"
        exit 0
    fi
done

printf 'Go %s is unavailable for %s at both official endpoints.\n' "$GO_VERSION" "$GO_ARCH" >&2
exit 1
