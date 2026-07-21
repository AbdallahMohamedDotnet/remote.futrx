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

OFFICIAL_AVAILABLE=0
for url in "${URLS[@]}"; do
    if curl --fail --silent --show-error --location --head \
        --connect-timeout 20 --retry 3 --retry-delay 2 "$url" >/dev/null; then
        OFFICIAL_AVAILABLE=1
        printf 'PASS: Go %s official archive is available for %s at %s\n' "$GO_VERSION" "$GO_ARCH" "$url"
        break
    fi
done

GITHUB_URL="$(go_toolchain_github_url "$GO_VERSION" "$GO_ARCH" || true)"
if [ -z "$GITHUB_URL" ]; then
    printf 'GitHub fallback is unavailable for Go %s on %s.\n' "$GO_VERSION" "$GO_ARCH" >&2
    exit 1
fi
if ! curl --fail --silent --show-error --location --head \
    --connect-timeout 20 --retry 3 --retry-delay 2 "$GITHUB_URL" >/dev/null; then
    printf 'GitHub fallback URL is unreachable: %s\n' "$GITHUB_URL" >&2
    exit 1
fi
printf 'PASS: Go %s GitHub fallback is available for %s at %s\n' "$GO_VERSION" "$GO_ARCH" "$GITHUB_URL"

if [ "$OFFICIAL_AVAILABLE" -eq 0 ]; then
    printf 'Official Go endpoints are unavailable; GitHub fallback remains healthy.\n'
fi
