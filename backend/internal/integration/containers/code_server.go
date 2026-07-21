package containers

// On-demand code-server: each project container runs its own code-server,
// socket-activated and idle-stopped (see templates/code-server-up.sh). New
// containers get it baked into the base image; EnsureCodeServer is the
// migration path for containers created before that image. Reached from the
// host edge at <slug>.code.<host> -> <slug>.lxd:8842, behind the same Google
// admin gate as the dev-URL proxy.

import (
	"context"
	_ "embed"
	"fmt"
	"time"
)

//go:embed templates/code-server-up.sh
var codeServerUpScript []byte

// EnsureCodeServer installs and enables the on-demand code-server stack inside
// an existing project container. Idempotent and best-effort, mirroring
// other container migration helpers. It returns early only when the socket is
// actually active; if the unit file exists but is disabled/stopped (e.g. a
// base-image bake that didn't enable it, or a unit that was turned off later)
// it still (re-)enables it, so a present-but-inert socket can't leave IDE
// routing silently broken.
func (c *Client) EnsureCodeServer(ctx context.Context, containerName, displayName string) error {
	// Fast path: socket already armed and listening -> nothing to do.
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := c.lxc.Run(cctx, "exec", containerName, "--", "systemctl", "is-active", "--quiet", "code-server.socket"); err == nil {
		return nil
	}

	// Install the units only when they're not present yet. The base image may
	// already ship them; re-running the install script is harmless but slow,
	// so skip it when the unit file exists and just (re-)enable below.
	tctx, tcancel := context.WithTimeout(ctx, 10*time.Second)
	defer tcancel()
	if _, err := c.lxc.Run(tctx, "exec", containerName, "--", "test", "-f", "/etc/systemd/system/code-server.socket"); err != nil {
		ictx, icancel := context.WithTimeout(ctx, 5*time.Minute)
		defer icancel()
		if out, err := c.lxc.Run(ictx, "exec", containerName, "--env", "CODE_SERVER_WS_NAME="+displayName, "--", "bash", "-c", string(codeServerUpScript)); err != nil {
			return fmt.Errorf("install code-server: %w; output: %s", err, truncateOut(out, 2000))
		}
	}

	// Always enable --now: arms a freshly-installed socket, and recovers a
	// baked-but-disabled/stopped one -- the case the old file-exists check
	// reported as complete while routing was actually dead.
	ectx, ecancel := context.WithTimeout(ctx, 20*time.Second)
	defer ecancel()
	if out, err := c.lxc.Run(ectx, "exec", containerName, "--", "systemctl", "enable", "--now", "code-server.socket"); err != nil {
		return fmt.Errorf("enable code-server.socket: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}
