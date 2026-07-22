// Package codeserver provisions the on-demand IDE inside project containers.
package codeserver

// On-demand code-server: each project container runs its own code-server,
// socket-activated and idle-stopped (see assets/code-server-up.sh). New
// containers get it baked into the base image; EnsureCodeServer is the
// migration path for containers created before that image. Reached from the
// host edge at <slug>.code.<host> -> <slug>.lxd:8842, behind the same Google
// admin gate as the dev-URL proxy.

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
	"github.com/futrx-com/remote.futrx.com/internal/shared/output"
)

//go:embed assets/code-server-up.sh
var codeServerUpScript []byte

// InstallScript returns the embedded code-server installation program used by
// base-image builds.
func InstallScript() []byte {
	return append([]byte(nil), codeServerUpScript...)
}

// Provisioner owns installation and socket activation for the
// per-container IDE.
type Provisioner struct {
	runner command.Runner
}

// NewProvisioner returns a code-server provisioner backed by runner.
func NewProvisioner(runner command.Runner) *Provisioner {
	return &Provisioner{runner: runner}
}

// EnsureCodeServer installs and enables the on-demand code-server stack inside
// an existing project container. Idempotent and best-effort, mirroring
// other container migration helpers. It returns early only when the socket is
// actually active; if the unit file exists but is disabled/stopped (e.g. a
// base-image bake that didn't enable it, or a unit that was turned off later)
// it still (re-)enables it, so a present-but-inert socket can't leave IDE
// routing silently broken.
func (p *Provisioner) Ensure(ctx context.Context, containerName, displayName string) error {
	// Fast path: socket already armed and listening -> nothing to do.
	if _, err := command.RunWithTimeout(ctx, p.runner, 10*time.Second, "exec", containerName, "--", "systemctl", "is-active", "--quiet", "code-server.socket"); err == nil {
		return nil
	}

	// Install the units only when they're not present yet. The base image may
	// already ship them; re-running the install script is harmless but slow,
	// so skip it when the unit file exists and just (re-)enable below.
	if _, err := command.RunWithTimeout(ctx, p.runner, 10*time.Second, "exec", containerName, "--", "test", "-f", "/etc/systemd/system/code-server.socket"); err != nil {
		if out, err := command.RunWithTimeout(ctx, p.runner, 5*time.Minute, "exec", containerName, "--env", "CODE_SERVER_WS_NAME="+displayName, "--", "bash", "-c", string(codeServerUpScript)); err != nil {
			return fmt.Errorf("install code-server: %w; output: %s", err, output.Truncate(out, 2000))
		}
	}

	// Always enable --now: arms a freshly-installed socket, and recovers a
	// baked-but-disabled/stopped one -- the case the old file-exists check
	// reported as complete while routing was actually dead.
	if out, err := command.RunWithTimeout(ctx, p.runner, 20*time.Second, "exec", containerName, "--", "systemctl", "enable", "--now", "code-server.socket"); err != nil {
		return fmt.Errorf("enable code-server.socket: %w; output: %s", err, output.Truncate(out, 1000))
	}
	return nil
}
