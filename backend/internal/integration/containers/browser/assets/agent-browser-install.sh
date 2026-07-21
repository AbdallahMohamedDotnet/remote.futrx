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
npx --yes playwright@1.60.0 install chromium 2>&1 | tail -3

# Sanity check the GUI toolchain (the chromium glob fails the build if absent).
which Xvfb x11vnc websockify openbox xdotool
ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome
