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
// env var. See agent/provisioning/assets/AGENTS.md for the user-facing recipe.

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

// templateHash is the canonical content hash used for the sha256 markers that
// gate template (re)pushes, and to check whether a pushed template is in sync.
func templateHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// pushTemplatedFile pushes content to each destPath inside the container with
// the given fileMode (e.g. "755"), gated by a single sha256 marker at
// hashPath — re-pushing only when the content has changed. Shared by every
// Ensure* step that ships an embedded template into the workspace, so the
// temp-file / push / marker dance lives in one place. Callers are responsible
// for creating any parent directories the destinations need.
func (c *Client) pushTemplatedFile(ctx context.Context, containerName string, content []byte, hashPath, fileMode string, destPaths ...string) error {
	want := templateHash(content)

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	got, err := c.lxc.Run(qctx, "exec", containerName, "--", "cat", hashPath)
	cancelQ()
	if err == nil && strings.TrimSpace(got) == want {
		return nil
	}

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	tmp, err := os.CreateTemp("", "futrx-template-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write template: %w", err)
	}
	tmp.Close()

	for _, destPath := range destPaths {
		if out, err := c.lxc.Run(pctx, "file", "push", "--mode="+fileMode, tmp.Name(), containerName+destPath); err != nil {
			return fmt.Errorf("push %s: %w; output: %s", destPath, err, out)
		}
	}
	if out, err := c.lxc.RunStdin(pctx, strings.NewReader(want), "exec", containerName, "--", "tee", hashPath); err != nil {
		return fmt.Errorf("write %s hash marker: %w; output: %s", hashPath, err, out)
	}
	return nil
}

// EnsureBrowserScript pushes the generic Playwright wrapper into the
// workspace and seeds an empty .agents/browser-auth.json if missing.
// Idempotent: the script is only re-pushed when its embedded content
// changes (sha256 marker stored alongside the config).
func (c *Client) EnsureBrowserScript(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}

	// Always ensure the directories + config file exist (cheap, idempotent).
	// We chmod 755 on dirs so the unprivileged container-root user can
	// traverse them; the host bind-mount preserves the uid 1000000 owner.
	dctx, cancelD := context.WithTimeout(ctx, 30*time.Second)
	_, err := c.lxc.Run(dctx, "exec", containerName, "--", "sh", "-c", `set -eu
mkdir -p /workspace/scripts /workspace/.agents /workspace/.browser
chmod 755 /workspace/scripts /workspace/.agents /workspace/.browser
if [ ! -f /workspace/.agents/browser-auth.json ]; then
  printf '{}\n' > /workspace/.agents/browser-auth.json
  chmod 644 /workspace/.agents/browser-auth.json
fi`)
	cancelD()
	if err != nil {
		return fmt.Errorf("seed browser dirs: %w", err)
	}

	return c.pushTemplatedFile(ctx, containerName, browserScriptTemplate, containerBrowserScriptHash, "755", containerBrowserScript)
}
