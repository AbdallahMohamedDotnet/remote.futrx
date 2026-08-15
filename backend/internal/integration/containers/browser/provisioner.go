package browser

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/assets"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
	"github.com/futrx-com/remote.futrx.com/internal/shared/output"
)

//go:embed assets/gui-up.sh
var guiUpScript []byte

//go:embed assets/human-input.sh
var humanInputScript []byte

const (
	containerGUIDir           = "/workspace/.browser-gui"
	containerGUIScript        = containerGUIDir + "/gui-up.sh"
	containerGUIScriptHash    = containerGUIDir + "/.gui-up.sha256"
	containerHumanInputScript = containerGUIDir + "/human-input.sh"
	containerHumanInputHash   = containerGUIDir + "/.human-input.sha256"

	agentBrowserInstallTimeout = 8 * time.Minute
)

// agentBrowserProvisioner owns browser package installation and the
// workspace-resident launcher templates.
type agentBrowserProvisioner struct {
	runner    command.Runner
	publisher *assets.Publisher
}

func (p *agentBrowserProvisioner) ensure(ctx context.Context, containerName string) error {
	if !p.runner.Available() {
		return command.ErrUnavailable
	}

	_, stackErr := command.RunWithTimeout(ctx, p.runner, queryTimeout, "exec", containerName, "--", "sh", "-c", browserStackCheck())
	if stackErr != nil {
		out, err := command.RunWithTimeout(ctx, p.runner, agentBrowserInstallTimeout, "exec", containerName, "--", "bash", "-c", InstallScript())
		if err != nil {
			return fmt.Errorf("install agent browser stack: %w; output: %s", err, output.TruncateTail(out, 2000))
		}
	}

	return p.pushTemplates(ctx, containerName)
}

func (p *agentBrowserProvisioner) pushTemplates(ctx context.Context, containerName string) error {
	out, err := command.RunWithTimeout(ctx, p.runner, queryTimeout, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	if err := p.publisher.Push(ctx, containerName, renderedGUIUpScript(), containerGUIScriptHash, "755", containerGUIScript); err != nil {
		return err
	}
	return p.publisher.Push(ctx, containerName, humanInputScript, containerHumanInputHash, "755", containerHumanInputScript)
}

func browserStackCheck() string {
	return `command -v Xvfb >/dev/null 2>&1 && for browser_bin in /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome; do [ -x "$browser_bin" ] || continue; "$browser_bin" --version 2>/dev/null | grep -Fq '` + provisioning.MustPin("PW_CFT_VERSION") + `' && exit 0; done; exit 1`
}

func renderedGUIUpScript() []byte {
	return bytes.ReplaceAll(
		guiUpScript,
		[]byte("__PW_CFT_VERSION__"),
		[]byte(provisioning.MustPin("PW_CFT_VERSION")),
	)
}
