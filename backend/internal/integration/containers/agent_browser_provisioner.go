package containers

import (
	"context"
	"errors"
	"fmt"
)

// agentBrowserProvisioner owns browser package installation and the
// workspace-resident launcher templates.
type agentBrowserProvisioner struct {
	lxc       CommandRunner
	templates *templatePublisher
}

func (p *agentBrowserProvisioner) ensure(ctx context.Context, containerName string) error {
	if !p.lxc.Available() {
		return errors.New("lxc not available")
	}

	cctx, cancel := context.WithTimeout(ctx, queryTimeout)
	_, stackErr := p.lxc.Run(cctx, "exec", containerName, "--", "sh", "-c", "command -v Xvfb >/dev/null 2>&1 && ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome >/dev/null 2>&1")
	cancel()
	if stackErr != nil {
		ictx, cancel := context.WithTimeout(ctx, agentBrowserInstallTimeout)
		out, err := p.lxc.Run(ictx, "exec", containerName, "--", "bash", "-c", agentBrowserInstallScript())
		cancel()
		if err != nil {
			return fmt.Errorf("install agent browser stack: %w; output: %s", err, truncateOut(out, 2000))
		}
	}

	return p.pushTemplates(ctx, containerName)
}

func (p *agentBrowserProvisioner) pushTemplates(ctx context.Context, containerName string) error {
	dctx, cancel := context.WithTimeout(ctx, queryTimeout)
	out, err := p.lxc.Run(dctx, "exec", containerName, "--", "install", "-d", "-m", "755", containerGUIDir)
	cancel()
	if err != nil {
		return fmt.Errorf("mkdir %s: %w; output: %s", containerGUIDir, err, out)
	}
	if err := p.templates.push(ctx, containerName, guiUpScript, containerGUIScriptHash, "755", containerGUIScript); err != nil {
		return err
	}
	return p.templates.push(ctx, containerName, humanInputScript, containerHumanInputHash, "755", containerHumanInputScript)
}
