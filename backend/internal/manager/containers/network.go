package containers

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
)

// RepairNetwork re-runs eth0 configuration (DHCP) inside the container.
// Safe on a healthy container: reconfigure just refreshes the same lease.
func (m *Manager) RepairNetwork(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	rctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if out, err := m.lxc.Run(rctx, "exec", containerName, "--", "networkctl", "reconfigure", "eth0"); err != nil {
		return fmt.Errorf("networkctl reconfigure eth0: %w; output: %s", err, out)
	}
	return nil
}
