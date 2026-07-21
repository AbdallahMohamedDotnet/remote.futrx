package containers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// cliProvisioner owns agent CLI readiness checks, installation, repair, and
// coalescing around installs already running inside a container.
type cliProvisioner struct {
	lxc      CommandRunner
	profiles *profileRegistry
}

// EnsureCLI is cheap on the normal path (one local `--version` call).
// Missing or stale CLIs are upgraded to the repository pin, and concurrent
// prompt starts coalesce around the npm install already running in the
// container.
func (c *Client) EnsureCLI(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	return c.clis.ensure(ctx, containerName, spec)
}

func (p *cliProvisioner) ensure(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
	if !p.lxc.Available() {
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
		installScript, scriptErr := baseImageInstallScript(p.profiles.snapshot())
		if scriptErr != nil {
			return fmt.Errorf("prepare agent CLI repair: %w", scriptErr)
		}
		out, err = p.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", installScript)
	} else if p.commandExists(installCtx, containerName, "npm") {
		out, err = p.lxc.Run(installCtx, "exec", containerName, "--",
			"npm", "install", "-g", spec.NPMPackage(), "--silent")
	} else {
		// Very old containers may pre-date Node/npm. Reuse the full image recipe
		// in that case so the runtime still self-heals from a bare rootfs.
		installScript, scriptErr := baseImageInstallScript(p.profiles.snapshot())
		if scriptErr != nil {
			return fmt.Errorf("prepare agent CLI repair: %w", scriptErr)
		}
		out, err = p.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", installScript)
	}
	if err != nil {
		waitCtx, cancelWait := context.WithTimeout(ctx, 90*time.Second)
		defer cancelWait()
		if waitErr := p.waitUntilReady(waitCtx, containerName, spec); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install %s in %s: %w; output: %s",
			cliInstallLabel(spec), containerName, err, truncateOut(out, 1000))
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

func (p *cliProvisioner) ready(ctx context.Context, containerName string, spec provisioning.CLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if !spec.CheckVersion {
		_, err := p.lxc.Run(quickCtx, "exec", containerName, "--", "which", spec.Binary)
		return err == nil
	}
	out, err := p.lxc.Run(quickCtx, "exec", containerName, "--", spec.Binary, "--version")
	return err == nil && semanticVersionAtLeast(out, spec.Version)
}

func (p *cliProvisioner) installRunning(ctx context.Context, containerName string, spec provisioning.CLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := p.lxc.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*"+spec.PackageName)
	return err == nil && strings.TrimSpace(out) != ""
}

func (p *cliProvisioner) commandExists(ctx context.Context, containerName, command string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := p.lxc.Run(quickCtx, "exec", containerName, "--", "which", command)
	return err == nil
}

func (p *cliProvisioner) waitUntilReady(ctx context.Context, containerName string, spec provisioning.CLISpec) error {
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

var semanticVersionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)(-([0-9A-Za-z.-]+))?`)

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	match := semanticVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return semanticVersion{}, false
	}
	major, errMajor := strconv.Atoi(match[1])
	minor, errMinor := strconv.Atoi(match[2])
	patch, errPatch := strconv.Atoi(match[3])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: match[5]}, true
}

func semanticVersionAtLeast(actualOutput, minimumValue string) bool {
	actual, ok := parseSemanticVersion(actualOutput)
	if !ok {
		return false
	}
	minimum, ok := parseSemanticVersion(minimumValue)
	if !ok {
		return false
	}
	actualCore := [3]int{actual.major, actual.minor, actual.patch}
	minimumCore := [3]int{minimum.major, minimum.minor, minimum.patch}
	for i := range actualCore {
		if actualCore[i] != minimumCore[i] {
			return actualCore[i] > minimumCore[i]
		}
	}
	if actual.prerelease == minimum.prerelease {
		return true
	}
	// A stable release is newer than a prerelease with the same core.
	if actual.prerelease == "" {
		return true
	}
	if minimum.prerelease == "" {
		return false
	}
	return actual.prerelease >= minimum.prerelease
}
