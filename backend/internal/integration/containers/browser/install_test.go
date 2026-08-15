package browser

import (
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

func TestInstallScriptSubstitutesEveryPin(t *testing.T) {
	script := InstallScript()
	if strings.Contains(script, "__PW_") || strings.Contains(script, "__PLAYWRIGHT_") {
		t.Fatalf("install script still contains unsubstituted placeholders:\n%s", script)
	}
	for _, want := range []string{
		"npx --yes \"playwright@${PLAYWRIGHT_VERSION}\"",
		"PW_CFT_VERSION=" + provisioning.MustPin("PW_CFT_VERSION"),
		"releases/download/",
		"sha256sum -c",
		"--enable-unsafe-swiftshader",
		"http://127.0.0.1:19222/json/version",
		"Chrome exited after initially opening CDP",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script is missing %q", want)
		}
	}
}
