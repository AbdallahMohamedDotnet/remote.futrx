package browser

import (
	_ "embed"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

//go:embed assets/agent-browser-install.sh
var embeddedAgentBrowserInstallScript string

//go:embed assets/agent-browser-smoke-test.sh
var embeddedAgentBrowserSmokeTestScript string

// InstallScript installs the headed-browser stack used by the Agent Browser
// feature. It is shared by base-image builds and the on-demand repair path
// for older containers. The __-delimited pins in the embedded asset are
// filled from versions.env so the Playwright version and the sha256-gated
// vendor fallback stay declared in one place.
func InstallScript() string {
	script := strings.TrimSuffix(embeddedAgentBrowserInstallScript, "\n") + "\n\n" +
		strings.TrimSuffix(embeddedAgentBrowserSmokeTestScript, "\n")
	for placeholder, key := range map[string]string{
		"__PLAYWRIGHT_VERSION__":               "PLAYWRIGHT_VERSION",
		"__PW_CFT_VERSION__":                   "PW_CFT_VERSION",
		"__PW_VENDOR_REPO__":                   "PW_VENDOR_REPO",
		"__PW_VENDOR_RELEASE_TAG__":            "PW_VENDOR_RELEASE_TAG",
		"__PW_CHROME_LINUX64_SHA256__":         "PW_CHROME_LINUX64_SHA256",
		"__PW_HEADLESS_SHELL_LINUX64_SHA256__": "PW_HEADLESS_SHELL_LINUX64_SHA256",
		"__PW_FFMPEG_LINUX_SHA256__":           "PW_FFMPEG_LINUX_SHA256",
	} {
		script = strings.ReplaceAll(script, placeholder, provisioning.MustPin(key))
	}
	return script
}
