package containers

// On-demand code-server: each project container runs its own code-server,
// socket-activated and idle-stopped (see templates/code-server-up.sh). New
// containers get it baked into the base image; EnsureCodeServer is the
// migration path for containers created before that image. Reached from the
// host edge at <slug>.code.<host> -> <slug>.lxd:8080, behind the same Google
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
// EnsureBrowserGUI: a no-op once the socket unit is present.
func (m *Manager) EnsureCodeServer(ctx context.Context, containerName string) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := m.lxc.Run(cctx, "exec", containerName, "--", "test", "-f", "/etc/systemd/system/code-server.socket"); err == nil {
		return nil
	}

	ictx, icancel := context.WithTimeout(ctx, 5*time.Minute)
	defer icancel()
	if out, err := m.lxc.Run(ictx, "exec", containerName, "--", "bash", "-c", string(codeServerUpScript)); err != nil {
		return fmt.Errorf("install code-server: %w; output: %s", err, truncateOut(out, 2000))
	}

	ectx, ecancel := context.WithTimeout(ctx, 20*time.Second)
	defer ecancel()
	if out, err := m.lxc.Run(ectx, "exec", containerName, "--", "systemctl", "enable", "--now", "code-server.socket"); err != nil {
		return fmt.Errorf("enable code-server.socket: %w; output: %s", err, truncateOut(out, 1000))
	}
	return nil
}
