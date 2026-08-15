set -e
export DEBIAN_FRONTEND=noninteractive
# Wait for the apt/dpkg lock rather than failing if apt-daily / unattended-
# upgrades is mid-run — a common race shortly after a container boots.
APT="apt-get -o DPkg::Lock::Timeout=300"
$APT update -qq

# Virtual display + VNC bridge (x11vnc -> websockify/noVNC over HTTP/WS), a
# lightweight window manager (openbox) so the browser window keeps stable
# input focus, and xdotool to activate that window. Font packages cover
# common web / CJK / emoji glyphs so real pages render legibly.
$APT install -y -qq \
    xvfb x11vnc novnc websockify openbox xdotool \
    libgtk-3-0t64 libgbm1 libasound2t64 libnss3 libxshmfence1 \
    dbus-x11 fonts-liberation fonts-noto-core fonts-noto-color-emoji

# Ubuntu 24.04 ships stock AppArmor profiles for browser binaries
# (/etc/apparmor.d/chrome, firefox, brave, ...) whose only purpose is to grant
# userns — but inside a nested LXD AppArmor namespace their network rules fail
# to match ("failed af match" in the host audit log), so a confined browser
# gets EVERY inet/inet6 socket create denied: no CDP socket, no page loads
# (CreatePlatformSocket: EPERM), while unconfined binaries (node, curl) work
# fine. Root-cause fix: an explicit allow-all network rule through the
# profile's local include (survives chrome package upgrades). Reload the
# profile if AppArmor is live so the on-demand install path takes effect
# immediately; at image-bake time profiles load on container boot anyway.
mkdir -p /etc/apparmor.d/local
echo "  network," > /etc/apparmor.d/local/chrome
if [ -f /etc/apparmor.d/chrome ] && command -v apparmor_parser >/dev/null 2>&1; then
    apparmor_parser -r /etc/apparmor.d/chrome 2>/dev/null || true
fi

# Browser: Playwright's Chromium (Chrome for Testing) — its install path
# (/root/.cache/ms-playwright) matches no AppArmor profile attachment, which
# is why it always networked while google-chrome-stable did not, and it is
# where gui-up.sh and browser.mjs both look. google-chrome-stable also works
# now thanks to the local include above; the Playwright build stays the
# baked-in default.
#
# __-delimited pins below are substituted from versions.env by
# backend/internal/integration/containers/browser/install.go.
PLAYWRIGHT_VERSION=__PLAYWRIGHT_VERSION__
PW_CFT_VERSION=__PW_CFT_VERSION__
VENDOR_URL="https://github.com/__PW_VENDOR_REPO__/releases/download/__PW_VENDOR_RELEASE_TAG__"

pw_install() {
    # pipefail so the tail filter cannot mask a failed install — a masked
    # failure here once surfaced 200 lines later as a bare "exit status 2".
    (set -o pipefail; npx --yes "playwright@${PLAYWRIGHT_VERSION}" install chromium 2>&1 | tail -20)
}

if ! pw_install; then
    # Google's Chrome-for-Testing CDN geo-blocks some datacenter IPs
    # (403 "not available in your location" — seen on Hetzner and Scaleway
    # ranges). Fall back to the project's own sha256-pinned copies of the
    # same archives, published to a GitHub release by
    # .github/workflows/vendor-playwright.yml. They are served to Playwright
    # from a loopback HTTP server so its installer (paths, revision dirs,
    # completion markers) runs untouched. See vendors/README.md.
    echo "direct Playwright download failed — retrying from vendored assets at ${VENDOR_URL}" >&2
    VENDOR_DIR=/tmp/pw-vendor
    mkdir -p "$VENDOR_DIR"
    for f in chrome-linux64.zip chrome-headless-shell-linux64.zip ffmpeg-linux.zip; do
        curl -fsSL --retry 3 -o "$VENDOR_DIR/$f" "$VENDOR_URL/$f"
    done
    sha256sum -c --quiet <<EOF
__PW_CHROME_LINUX64_SHA256__  $VENDOR_DIR/chrome-linux64.zip
__PW_HEADLESS_SHELL_LINUX64_SHA256__  $VENDOR_DIR/chrome-headless-shell-linux64.zip
__PW_FFMPEG_LINUX_SHA256__  $VENDOR_DIR/ffmpeg-linux.zip
EOF
    # Serve the archives by filename for whatever path Playwright requests —
    # its URL layout under a custom PLAYWRIGHT_DOWNLOAD_HOST has changed
    # between releases; the basenames have not.
    node -e '
        const http = require("http"), fs = require("fs"), path = require("path");
        const dir = process.argv[1];
        http.createServer((req, res) => {
            const name = path.basename(new URL(req.url, "http://localhost").pathname);
            const file = path.join(dir, name);
            if (!/^[A-Za-z0-9._-]+$/.test(name) || !fs.existsSync(file)) {
                res.statusCode = 404;
                return res.end("not vendored: " + name);
            }
            res.setHeader("content-length", fs.statSync(file).size);
            fs.createReadStream(file).pipe(res);
        }).listen(8377, "127.0.0.1");
    ' "$VENDOR_DIR" &
    VENDOR_SRV=$!
    trap 'kill "$VENDOR_SRV" 2>/dev/null || true' EXIT
    for _ in $(seq 1 50); do
        curl -s -o /dev/null "http://127.0.0.1:8377/" && break
        sleep 0.2
    done
    if ! PLAYWRIGHT_DOWNLOAD_HOST=http://127.0.0.1:8377 pw_install; then
        echo "Playwright install failed from both Google's CDN and the vendored release." >&2
        echo "Likely causes: this server's IP is geo-blocked by Google AND ${VENDOR_URL} is" >&2
        echo "missing assets for playwright@${PLAYWRIGHT_VERSION}. See vendors/README.md." >&2
        exit 1
    fi
    kill "$VENDOR_SRV" 2>/dev/null || true
    trap - EXIT
    rm -rf "$VENDOR_DIR"
fi

# Sanity check the GUI toolchain and select the browser pinned by versions.env.
which Xvfb x11vnc websockify openbox xdotool
CHROME=""
for browser_bin in /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome; do
    [ -x "$browser_bin" ] || continue
    if "$browser_bin" --version 2>/dev/null | grep -Fq "$PW_CFT_VERSION"; then
        CHROME="$browser_bin"
        break
    fi
done
if [ -z "$CHROME" ]; then
    echo "pinned Chrome for Testing $PW_CFT_VERSION was not installed" >&2
    exit 1
fi

# A present executable is insufficient: the 0.3.0 image shipped a browser that
# opened CDP and then immediately died with SIGTRAP. Keep the exact production
# launch flags alive long enough to prove the browser core is usable before an
# image can be published.
SMOKE_DIR="$(mktemp -d /tmp/remote-browser-smoke.XXXXXX)"
SMOKE_X_PID=""
SMOKE_CHROME_PID=""
smoke_cleanup() {
    if [ -n "$SMOKE_CHROME_PID" ]; then
        kill "$SMOKE_CHROME_PID" 2>/dev/null || true
        wait "$SMOKE_CHROME_PID" 2>/dev/null || true
    fi
    if [ -n "$SMOKE_X_PID" ]; then
        kill "$SMOKE_X_PID" 2>/dev/null || true
        wait "$SMOKE_X_PID" 2>/dev/null || true
    fi
    rm -rf -- "$SMOKE_DIR"
}
trap smoke_cleanup EXIT

Xvfb -displayfd 3 -screen 0 1366x768x24 -ac -nolisten tcp \
    3>"$SMOKE_DIR/display" >"$SMOKE_DIR/xvfb.log" 2>&1 &
SMOKE_X_PID=$!
for _ in $(seq 1 50); do
    [ -s "$SMOKE_DIR/display" ] && break
    kill -0 "$SMOKE_X_PID" 2>/dev/null || break
    sleep 0.1
done
if [ ! -s "$SMOKE_DIR/display" ]; then
    echo "browser smoke test could not start Xvfb" >&2
    tail -40 "$SMOKE_DIR/xvfb.log" >&2 || true
    exit 1
fi

DISPLAY=":$(cat "$SMOKE_DIR/display")" "$CHROME" \
    --user-data-dir="$SMOKE_DIR/profile" \
    --no-sandbox --no-first-run --no-default-browser-check \
    --disable-dev-shm-usage \
    --use-gl=angle --use-angle=swiftshader-webgl --enable-unsafe-swiftshader \
    --renderer-process-limit=4 \
    --disable-background-networking \
    --disable-features=Translate,MediaRouter,OptimizationHints \
    --metrics-recording-only --mute-audio \
    --remote-debugging-port=19222 \
    --window-position=0,0 --window-size=1366,768 \
    about:blank >"$SMOKE_DIR/chrome.log" 2>&1 &
SMOKE_CHROME_PID=$!

SMOKE_READY=0
for _ in $(seq 1 30); do
    if curl -sf --max-time 1 http://127.0.0.1:19222/json/version >/dev/null 2>&1; then
        SMOKE_READY=1
        break
    fi
    if ! kill -0 "$SMOKE_CHROME_PID" 2>/dev/null; then
        break
    fi
    sleep 1
done
if [ "$SMOKE_READY" -ne 1 ]; then
    echo "browser smoke test failed: Chrome did not keep CDP port 19222 ready" >&2
    tail -80 "$SMOKE_DIR/chrome.log" >&2 || true
    exit 1
fi

# Catch the observed starts-then-SIGTRAP failure rather than accepting a
# momentary successful curl.
sleep 3
if ! kill -0 "$SMOKE_CHROME_PID" 2>/dev/null || \
   ! curl -sf --max-time 1 http://127.0.0.1:19222/json/version >/dev/null 2>&1; then
    echo "browser smoke test failed: Chrome exited after initially opening CDP" >&2
    tail -80 "$SMOKE_DIR/chrome.log" >&2 || true
    exit 1
fi

smoke_cleanup
trap - EXIT
echo "Agent Browser smoke test passed with Chrome for Testing $PW_CFT_VERSION"
