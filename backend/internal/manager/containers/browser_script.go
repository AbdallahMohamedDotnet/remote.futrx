package containers

// Browser-script provisioning: pushes the generic Playwright wrapper at
// /workspace/scripts/browser.mjs and seeds an empty config file at
// /workspace/.agents/browser-auth.json. Both are workspace-resident so
// they survive container deletes; the host re-pushes the script whenever
// the embedded template changes (sha256 marker, same pattern as AGENTS.md).
//
// The script reads .agents/browser-auth.json to know which cookie to
// attach per host; the agent edits that file when adding a new site, and
// the user pastes the cookie value into the Secrets UI under the named
// env var. See templates/AGENTS.md for the user-facing recipe.

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

//go:embed templates/browser.mjs
var browserScriptTemplate []byte

const (
	containerBrowserScript     = "/workspace/scripts/browser.mjs"
	containerBrowserAuthConfig = "/workspace/.agents/browser-auth.json"
	containerBrowserScriptHash = "/workspace/.agents/.browser-mjs.sha256"
)

func browserScriptHash() string {
	sum := sha256.Sum256(browserScriptTemplate)
	return hex.EncodeToString(sum[:])
}

// EnsureBrowserScript pushes the generic Playwright wrapper into the
// workspace and seeds an empty .agents/browser-auth.json if missing.
// Idempotent: the script is only re-pushed when its embedded content
// changes (sha256 marker stored alongside the config).
func (m *Manager) EnsureBrowserScript(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	want := browserScriptHash()

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	defer cancelQ()
	got, err := m.lxc.Run(qctx, "exec", containerName, "--", "cat", containerBrowserScriptHash)
	scriptCurrent := err == nil && strings.TrimSpace(got) == want

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	// Always ensure the directories + config file exist (cheap, idempotent).
	// We chmod 755 on dirs so the unprivileged container-root user can
	// traverse them; the host bind-mount preserves the uid 1000000 owner.
	if _, err := m.lxc.Run(pctx, "exec", containerName, "--", "sh", "-c", `set -eu
mkdir -p /workspace/scripts /workspace/.agents /workspace/.browser
chmod 755 /workspace/scripts /workspace/.agents /workspace/.browser
if [ ! -f /workspace/.agents/browser-auth.json ]; then
  printf '{}\n' > /workspace/.agents/browser-auth.json
  chmod 644 /workspace/.agents/browser-auth.json
fi`); err != nil {
		return fmt.Errorf("seed browser dirs: %w", err)
	}

	if scriptCurrent {
		return nil
	}

	tmp, err := os.CreateTemp("", "browser-mjs-*.mjs")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(browserScriptTemplate); err != nil {
		tmp.Close()
		return fmt.Errorf("write template: %w", err)
	}
	tmp.Close()

	if out, err := m.lxc.Run(pctx, "file", "push", "--mode=755",
		tmp.Name(), containerName+containerBrowserScript); err != nil {
		return fmt.Errorf("push browser.mjs: %w; output: %s", err, out)
	}
	if out, err := m.lxc.RunStdin(pctx, strings.NewReader(want), "exec", containerName, "--",
		"tee", containerBrowserScriptHash); err != nil {
		return fmt.Errorf("write browser.mjs hash marker: %w; output: %s", err, out)
	}
	return nil
}
