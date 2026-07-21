package browser

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/output"
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
		return errors.New("lxc not available")
	}

	cctx, cancel := context.WithTimeout(ctx, queryTimeout)
	_, stackErr := p.runner.Run(cctx, "exec", containerName, "--", "sh", "-c", "command -v Xvfb >/dev/null 2>&1 && ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome >/dev/null 2>&1")
	cancel()
	if stackErr != nil {
		ictx, cancel := context.WithTimeout(ctx, agentBrowserInstallTimeout)
		out, err := p.runner.Run(ictx, "exec", containerName, "--", "bash", "-c", InstallScript())
		cancel()
		if err != nil {
			return fmt.Errorf("install agent browser stack: %w; output: %s", err, output.Truncate(out, 2000))
		}
	}

	return p.pushTemplates(ctx, containerName)
}

func (p *agentBrowserProvisioner) pushTemplates(ctx context.Context, containerName string) error {
	dctx, cancel := context.WithTimeout(ctx, queryTimeout)
	out, err := p.runner.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancel()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	if err := p.publisher.Push(ctx, containerName, guiUpScript, containerGUIScriptHash, "755", containerGUIScript); err != nil {
		return err
	}
	return p.publisher.Push(ctx, containerName, humanInputScript, containerHumanInputHash, "755", containerHumanInputScript)
}
