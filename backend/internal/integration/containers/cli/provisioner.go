// Package cli provisions and repairs agent command-line tools in project
// containers.
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/output"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
)

const queryTimeout = 10 * time.Second

type repairRecipe func([]provisioning.Profile) (string, error)

// cliProvisioner owns agent CLI readiness checks, installation, repair, and
// coalescing around installs already running inside a container.
type Provisioner struct {
	runner       command.Runner
	profiles     *profiles.Registry
	repairRecipe repairRecipe
}

// NewProvisioner returns an agent CLI provisioner backed by the shared
// profile registry and full-image repair recipe.
func NewProvisioner(
	runner command.Runner,
	registry *profiles.Registry,
	recipe func([]provisioning.Profile) (string, error),
) *Provisioner {
	return &Provisioner{runner: runner, profiles: registry, repairRecipe: recipe}
}

// EnsureCLI is cheap on the normal path (one local `--version` call).
// Missing or stale CLIs are upgraded to the repository pin, and concurrent
// prompt starts coalesce around the npm install already running in the
// container.
func (p *Provisioner) Ensure(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	if !p.runner.Available() {
		return errors.New("lxc not available")
	}
	if p.ready(ctx, containerName, spec) {
		return nil
	}
	if p.installRunning(ctx, containerName, spec) {
		waitCtx, cancel := context.WithTimeout(ctx, spec.WaitTimeout)
		defer cancel()
		if err := p.waitUntilReady(waitCtx, containerName, spec); err == nil {
			return nil
		}
	}

	installCtx, cancel := context.WithTimeout(ctx, spec.InstallTimeout)
	defer cancel()

	var out string
	var err error
	if spec.InstallMode == provisioning.InstallWithImageRepair {
		installScript, scriptErr := p.repairRecipe(p.profiles.Snapshot())
		if scriptErr != nil {
			return fmt.Errorf("prepare agent CLI repair: %w", scriptErr)
		}
		out, err = p.runner.Run(installCtx, "exec", containerName, "--", "bash", "-c", installScript)
	} else if p.commandExists(installCtx, containerName, "npm") {
		out, err = p.runner.Run(installCtx, "exec", containerName, "--",
			"npm", "install", "-g", spec.NPMPackage(), "--silent")
	} else {
		// Very old containers may pre-date Node/npm. Reuse the full image recipe
		// in that case so the runtime still self-heals from a bare rootfs.
		installScript, scriptErr := p.repairRecipe(p.profiles.Snapshot())
		if scriptErr != nil {
			return fmt.Errorf("prepare agent CLI repair: %w", scriptErr)
		}
		out, err = p.runner.Run(installCtx, "exec", containerName, "--", "bash", "-c", installScript)
	}
	if err != nil {
		waitCtx, cancelWait := context.WithTimeout(ctx, 90*time.Second)
		defer cancelWait()
		if waitErr := p.waitUntilReady(waitCtx, containerName, spec); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install %s in %s: %w; output: %s",
			cliInstallLabel(spec), containerName, err, output.Truncate(out, 1000))
	}
	if spec.VerifyAfterInstall && !p.ready(ctx, containerName, spec) {
		return fmt.Errorf("install %s in %s completed but the required version is unavailable",
			cliInstallLabel(spec), containerName)
	}
	return nil
}

func cliInstallLabel(spec provisioning.CLISpec) string {
	if spec.ReportVersion && spec.Version != "" {
		return spec.Name + " " + spec.Version
	}
	return spec.Name
}

func (p *Provisioner) ready(ctx context.Context, containerName string, spec provisioning.CLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if !spec.CheckVersion {
		_, err := p.runner.Run(quickCtx, "exec", containerName, "--", "which", spec.Binary)
		return err == nil
	}
	out, err := p.runner.Run(quickCtx, "exec", containerName, "--", spec.Binary, "--version")
	return err == nil && semanticVersionAtLeast(out, spec.Version)
}

func (p *Provisioner) installRunning(ctx context.Context, containerName string, spec provisioning.CLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := p.runner.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*"+spec.PackageName)
	return err == nil && strings.TrimSpace(out) != ""
}

func (p *Provisioner) commandExists(ctx context.Context, containerName, command string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := p.runner.Run(quickCtx, "exec", containerName, "--", "which", command)
	return err == nil
}

func (p *Provisioner) waitUntilReady(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if p.ready(ctx, containerName, spec) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
