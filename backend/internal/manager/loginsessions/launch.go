package loginsessions

// launch.go: builds the bash command we shell out to inside the project's
// LXD container to spin up a headless Chromium with --remote-debugging-port
// exposed on 0.0.0.0. Kept separate from manager.go so the giant shell
// blob is easy to find and tweak.

import (
	"fmt"
	"strings"
)

type launchScriptArgs struct {
	Port        int
	UserDataDir string
	URL         string
}

// buildLaunchScript returns a bash -c friendly command that:
//   - Ensures Chromium is available (Playwright's bundled build),
//   - Installs system deps if missing (best-effort),
//   - Backgrounds Chromium with --remote-debugging-port on 0.0.0.0,
//   - Returns immediately; the caller then polls the DevTools endpoint.
//
// Important: we deliberately avoid here-docs and backticks; all strings are
// joined with a couple of helpers so the script is safe to scp upload and
// embed in `lxc exec -- bash -lc <script>`.
func buildLaunchScript(args launchScriptArgs) string {
	port := args.Port
	dir := shellQuote(args.UserDataDir)
	url := shellQuote(args.URL)

	// System packages Chromium needs. Best-effort: if apt-get fails we still
	// try to launch — the user will see the error in the screencast.
	aptPkgs := []string{
		"libnss3", "libglib2.0-0", "libnspr4", "libdbus-1-3",
		"libatk1.0-0", "libatk-bridge2.0-0", "libcups2", "libdrm2",
		"libxkbcommon0", "libxcomposite1", "libxdamage1", "libxfixes3",
		"libxrandr2", "libgbm1", "libpango-1.0-0", "libcairo2",
		"libasound2t64", "fonts-liberation",
	}

	parts := []string{
		"set -e",
		"export NPM_CONFIG_PREFIX=/root/.npm-global",
		"export PATH=/root/.npm-global/bin:$PATH",
		"mkdir -p /root/.npm-global",
		// Lazy install playwright if the chromium binary it bundles is missing.
		"if ! ls /root/.cache/ms-playwright/chromium-*/chrome-linux/chrome >/dev/null 2>&1; then " +
			"DEBIAN_FRONTEND=noninteractive apt-get -qq update >/tmp/login-apt.log 2>&1 || true; " +
			"DEBIAN_FRONTEND=noninteractive apt-get -qq install -y --no-install-recommends " +
			strings.Join(aptPkgs, " ") + " >>/tmp/login-apt.log 2>&1 || true; " +
			"npm install -g playwright >>/tmp/login-apt.log 2>&1 || true; " +
			"npx --yes playwright install chromium >>/tmp/login-apt.log 2>&1; " +
			"fi",
		// Discover the chromium binary path that playwright unpacked.
		"CHROME_BIN=$(ls /root/.cache/ms-playwright/chromium-*/chrome-linux/chrome 2>/dev/null | head -n1)",
		"if [ -z \"$CHROME_BIN\" ]; then echo 'chromium not installed'; cat /tmp/login-apt.log 2>/dev/null | tail -40; exit 1; fi",
		fmt.Sprintf("mkdir -p %s", dir),
		// Launch Chromium headless-new with remote debugging exposed on the
		// container interface so the host backend can reach it via slug.lxd.
		// Disable Chromium-side first-run + signin nudges.
		"nohup \"$CHROME_BIN\" " +
			fmt.Sprintf("--remote-debugging-port=%d ", port) +
			"--remote-debugging-address=0.0.0.0 " +
			"--headless=new " +
			fmt.Sprintf("--user-data-dir=%s ", dir) +
			"--no-first-run " +
			"--no-default-browser-check " +
			"--disable-features=Translate,InfiniteSessionRestore " +
			"--window-size=1280,720 " +
			"--hide-scrollbars=false " +
			fmt.Sprintf("--no-sandbox %s ", url) +
			fmt.Sprintf(">/tmp/login-%d.log 2>&1 &", port),
		"disown || true",
		// Tiny delay so the parent shell doesn't return before chromium has
		// bound its socket. The real readiness check is the DevTools poll.
		"sleep 0.2",
	}
	return strings.Join(parts, "; ")
}
