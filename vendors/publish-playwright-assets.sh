#!/usr/bin/env bash
# Publishes the Playwright browser archives pinned in versions.env as GitHub
# release assets. They are the install-time fallback for servers whose IPs
# Google's Chrome-for-Testing CDN geo-blocks (403 "not available in your
# location" — seen on Hetzner and Scaleway ranges). See vendors/README.md.
#
# Run from any machine with clean egress (or let the "Vendor Playwright
# assets" workflow run it on a GitHub runner):
#   bash vendors/publish-playwright-assets.sh
#
# Requires: curl, node/npx, gh (authenticated with write access to the repo).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
. "$ROOT/infra/versions.env"
for v in PLAYWRIGHT_VERSION PW_CFT_VERSION PW_FFMPEG_BUILD PW_VENDOR_REPO \
         PW_VENDOR_RELEASE_TAG PW_CHROME_LINUX64_SHA256 \
         PW_HEADLESS_SHELL_LINUX64_SHA256 PW_FFMPEG_LINUX_SHA256; do
    [ -n "${!v:-}" ] || { echo "versions.env is missing $v" >&2; exit 1; }
done

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
    else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# ── Consistency gate ─────────────────────────────────────────────────────────
# The CfT and ffmpeg pins must match what the pinned Playwright actually
# requests, or the vendored assets would never be the ones the installer asks
# for. (Version numbers are platform-independent; the dry-run may print this
# machine's platform in URLs.)
dryrun="$(npx --yes "playwright@${PLAYWRIGHT_VERSION}" install chromium --dry-run 2>/dev/null)"
want_cft="$(printf '%s\n' "$dryrun" | sed -n 's/.*Chrome for Testing \([0-9.]*\) .*/\1/p' | head -1)"
want_ffmpeg="$(printf '%s\n' "$dryrun" | sed -n 's/.*playwright ffmpeg v\([0-9]*\).*/\1/p' | head -1)"
if [ "$want_cft" != "$PW_CFT_VERSION" ] || [ "$want_ffmpeg" != "$PW_FFMPEG_BUILD" ]; then
    echo "pin mismatch: playwright@${PLAYWRIGHT_VERSION} wants CfT ${want_cft:-?} / ffmpeg ${want_ffmpeg:-?}," >&2
    echo "but versions.env pins PW_CFT_VERSION=${PW_CFT_VERSION} / PW_FFMPEG_BUILD=${PW_FFMPEG_BUILD}." >&2
    echo "Update versions.env (and its sha256 pins) before publishing." >&2
    exit 1
fi

# ── Download the three Linux assets ──────────────────────────────────────────
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fetch() { # fetch <output-name> <url>...
    local out="$WORK/$1"; shift
    local url
    for url in "$@"; do
        echo "  <- $url"
        if curl -fsSL --retry 3 -o "$out" "$url"; then return 0; fi
        echo "     (failed, trying next source)"
    done
    echo "could not download $1 from any source" >&2
    return 1
}

echo "downloading assets for playwright@${PLAYWRIGHT_VERSION} (CfT ${PW_CFT_VERSION}, ffmpeg ${PW_FFMPEG_BUILD})"
fetch chrome-linux64.zip \
    "https://cdn.playwright.dev/builds/cft/${PW_CFT_VERSION}/linux64/chrome-linux64.zip"
fetch chrome-headless-shell-linux64.zip \
    "https://cdn.playwright.dev/builds/cft/${PW_CFT_VERSION}/linux64/chrome-headless-shell-linux64.zip"
fetch ffmpeg-linux.zip \
    "https://cdn.playwright.dev/dbazure/download/playwright/builds/ffmpeg/${PW_FFMPEG_BUILD}/ffmpeg-linux.zip" \
    "https://playwright.download.prss.microsoft.com/dbazure/download/playwright/builds/ffmpeg/${PW_FFMPEG_BUILD}/ffmpeg-linux.zip"

# ── Verify against the committed pins ────────────────────────────────────────
fail=0
check() { # check <file> <pinned-sha>
    local got; got="$(sha256 "$WORK/$1")"
    if [ "$got" != "$2" ]; then
        echo "sha256 mismatch for $1:" >&2
        echo "  pinned: $2" >&2
        echo "  actual: $got" >&2
        fail=1
    fi
}
check chrome-linux64.zip "$PW_CHROME_LINUX64_SHA256"
check chrome-headless-shell-linux64.zip "$PW_HEADLESS_SHELL_LINUX64_SHA256"
check ffmpeg-linux.zip "$PW_FFMPEG_LINUX_SHA256"
if [ "$fail" -ne 0 ]; then
    echo "If you just bumped PLAYWRIGHT_VERSION, update the PW_*_SHA256 pins in" >&2
    echo "versions.env to the 'actual' values above and re-run." >&2
    exit 1
fi
echo "sha256 pins verified"

# ── Publish (create-or-update; the tag itself is never moved) ────────────────
if ! gh release view "$PW_VENDOR_RELEASE_TAG" -R "$PW_VENDOR_REPO" >/dev/null 2>&1; then
    gh release create "$PW_VENDOR_RELEASE_TAG" -R "$PW_VENDOR_REPO" \
        --title "Vendored Playwright assets (playwright@${PLAYWRIGHT_VERSION})" \
        --notes "Unmodified Playwright/Chrome-for-Testing archives (CfT ${PW_CFT_VERSION}, ffmpeg ${PW_FFMPEG_BUILD}), republished as the install-time fallback for servers geo-blocked by Google's CDN. sha256 pins live in versions.env; provenance is this repo's 'Vendor Playwright assets' workflow. See vendors/README.md."
fi
gh release upload "$PW_VENDOR_RELEASE_TAG" -R "$PW_VENDOR_REPO" --clobber \
    "$WORK/chrome-linux64.zip" \
    "$WORK/chrome-headless-shell-linux64.zip" \
    "$WORK/ffmpeg-linux.zip"

# ── Round-trip: the URLs the install fallback will use must serve the bytes ──
base="https://github.com/${PW_VENDOR_REPO}/releases/download/${PW_VENDOR_RELEASE_TAG}"
for f in chrome-linux64.zip chrome-headless-shell-linux64.zip ffmpeg-linux.zip; do
    curl -fsSL --retry 3 -o "$WORK/rt-$f" "$base/$f"
    [ "$(sha256 "$WORK/rt-$f")" = "$(sha256 "$WORK/$f")" ] \
        || { echo "round-trip sha mismatch for $f" >&2; exit 1; }
done
echo "published and verified: $base"
