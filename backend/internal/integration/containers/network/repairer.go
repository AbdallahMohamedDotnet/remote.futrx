// Package network repairs project-container connectivity.
package network

// Network repair: under host load, in-container systemd-networkd can hit
// rtnl timeouts, mark eth0 failed and flush its DHCP address + routes
// without ever retrying — leaving a RUNNING container with IPv6 link-local
// only, so agents die with "Network unreachable (os error 101)". A
// host-side sweep (lxc-ipv4-heal.timer) catches this within a minute; the
// manual path below backs it and is exposed in the workspace info UI.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
)

const queryTimeout = 10 * time.Second

// Repairer owns safe container network recovery.
type Repairer struct {
	runner command.Runner
}

// NewRepairer returns a network repairer backed by runner.
func NewRepairer(runner command.Runner) *Repairer {
	return &Repairer{runner: runner}
}

// RepairNetwork re-runs eth0 configuration (DHCP) inside the container.
// Safe on a healthy container: reconfigure just refreshes the same lease.
func (r *Repairer) Repair(ctx context.Context, containerName string) error {
	if !r.runner.Available() {
		return errors.New("lxc not available")
	}
	rctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if out, err := r.runner.Run(rctx, "exec", containerName, "--", "networkctl", "reconfigure", "eth0"); err != nil {
		return fmt.Errorf("networkctl reconfigure eth0: %w; output: %s", err, out)
	}
	return nil
}
