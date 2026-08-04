package browser

import (
	"strings"
	"testing"
)

func TestInstallScriptSubstitutesEveryPin(t *testing.T) {
	script := InstallScript()
	if strings.Contains(script, "__PW_") || strings.Contains(script, "__PLAYWRIGHT_") {
		t.Fatalf("install script still contains unsubstituted placeholders:\n%s", script)
	}
	for _, want := range []string{
		"npx --yes \"playwright@${PLAYWRIGHT_VERSION}\"",
		"releases/download/",
		"sha256sum -c",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script is missing %q", want)
		}
	}
}
