package loginsessions

// launch.go: builds the bash command we shell out to inside the project's
// LXD container to spin up a headless Chromium with --remote-debugging-port
// plus a socat forwarder to expose it on the container interface. Kept
// separate from manager.go so the giant shell blob is easy to find and
// tweak.

import (
	"fmt"
	"strings"
)

type launchScriptArgs struct {
	ChromePort  int // chromium binds here on 127.0.0.1
	ProxyPort   int // socat listens here on 0.0.0.0, forwards to ChromePort
	UserDataDir string
	URL         string
}

// buildLaunchScript returns a bash -c friendly command that:
//   - Ensures Chromium and socat are installed (best-effort apt install),
//   - Lazy-installs Playwright + chromium if not already present,
//   - Backgrounds Chromium with --remote-debugging-port on 127.0.0.1,
//   - Backgrounds socat to forward 0.0.0.0:ProxyPort → 127.0.0.1:ChromePort
//     (necessary because Chromium silently ignores --remote-debugging-address
//     as a security measure),
//   - Returns immediately; the caller then polls the DevTools endpoint.
//
// Important: we deliberately avoid here-docs and backticks; all strings are
// joined with a couple of helpers so the script is safe to scp upload and
// embed in `lxc exec -- bash -lc <script>`.
func buildLaunchScript(args launchScriptArgs) string {
	chromePort := args.ChromePort
	proxyPort := args.ProxyPort
	dir := shellQuote(args.UserDataDir)
	url := shellQuote(args.URL)

	// System packages Chromium needs. Best-effort: if apt-get fails we still
	// try to launch — the user will see the error in the screencast.
	aptPkgs := []string{
		"libnss3", "libglib2.0-0", "libnspr4", "libdbus-1-3",
		"libatk1.0-0", "libatk-bridge2.0-0", "libcups2", "libdrm2",
		"libxkbcommon0", "libxcomposite1", "libxdamage1", "libxfixes3",
		"libxrandr2", "libgbm1", "libpango-1.0-0", "libcairo2",
		"libasound2t64", "fonts-liberation", "socat",
	}

	parts := []string{
		"set -e",
		"export NPM_CONFIG_PREFIX=/root/.npm-global",
		"export PATH=/root/.npm-global/bin:$PATH",
		"mkdir -p /root/.npm-global",
		// Always make sure socat is installed; it's not part of the base
		// image. Chromium deps + playwright only on cold install.
		"if ! command -v socat >/dev/null 2>&1 || ! ls /root/.cache/ms-playwright/chromium-*/chrome-linux*/chrome >/dev/null 2>&1; then " +
			"DEBIAN_FRONTEND=noninteractive apt-get -qq update >/tmp/login-apt.log 2>&1 || true; " +
			"DEBIAN_FRONTEND=noninteractive apt-get -qq install -y --no-install-recommends " +
			strings.Join(aptPkgs, " ") + " >>/tmp/login-apt.log 2>&1 || true; " +
			"fi",
		"if ! ls /root/.cache/ms-playwright/chromium-*/chrome-linux*/chrome >/dev/null 2>&1; then " +
			"npm install -g playwright >>/tmp/login-apt.log 2>&1 || true; " +
			"npx --yes playwright install chromium >>/tmp/login-apt.log 2>&1; " +
			"fi",
		// Discover the chromium binary path that playwright unpacked.
		"CHROME_BIN=$(ls /root/.cache/ms-playwright/chromium-*/chrome-linux*/chrome 2>/dev/null | head -n1)",
		"if [ -z \"$CHROME_BIN\" ]; then echo 'chromium not installed'; cat /tmp/login-apt.log 2>/dev/null | tail -40; exit 1; fi",
		"if ! command -v socat >/dev/null 2>&1; then echo 'socat not installed'; exit 1; fi",
		fmt.Sprintf("mkdir -p %s", dir),
		// Launch Chromium headless-new bound to 127.0.0.1. The socat
		// forwarder below makes it reachable from outside the container.
		// nohup + background; trailing `&` lets us join with `; ` safely
		// because we wrap the command in a `( ... )& ` subshell.
		"(nohup \"$CHROME_BIN\" " +
			fmt.Sprintf("--remote-debugging-port=%d ", chromePort) +
			"--remote-allow-origins=* " +
			"--headless=new " +
			fmt.Sprintf("--user-data-dir=%s ", dir) +
			"--no-first-run " +
			"--no-default-browser-check " +
			"--disable-features=Translate,InfiniteSessionRestore " +
			"--window-size=1280,720 " +
			fmt.Sprintf("--no-sandbox %s ", url) +
			fmt.Sprintf(">/tmp/login-chrome-%d.log 2>&1 )&", chromePort),
		// socat: forward 0.0.0.0:ProxyPort → 127.0.0.1:ChromePort. Without
		// this the host can reach the container by hostname but Chromium
		// only listens on loopback (it ignores --remote-debugging-address
		// as a security measure against DNS rebinding).
		fmt.Sprintf(
			"(nohup socat TCP-LISTEN:%d,fork,bind=0.0.0.0,reuseaddr TCP:127.0.0.1:%d "+
				">/tmp/login-socat-%d.log 2>&1 )&",
			proxyPort, chromePort, proxyPort),
		// Tiny delay so the parent shell doesn't return before chromium /
		// socat have bound their sockets. The real readiness check is
		// the DevTools poll.
		"sleep 0.3",
	}
	// Newline join (not `; `) so commands ending in `&` (the chromium
	// + socat backgrounds) don't collide with a following `;` — bash
	// considers `& ; cmd` a syntax error.
	return strings.Join(parts, "\n")
}
