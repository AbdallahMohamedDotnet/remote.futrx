package containers

// ApplyContainerEnvDiff drives LXD's built-in environment.* config keys.
// Used by the project-secrets flow to push per-project secrets into the
// project's container, so any subsequent `lxc exec` (every Claude prompt,
// every interactive shell) inherits the var automatically.

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ApplyContainerEnvDiff sets the values in set and removes the keys in unset.
// Idempotent: a set that already matches the current value is a no-op (cheap
// read before write). Returns the first error encountered.
//
// Either map can be nil/empty. Already-running processes inside the container
// keep their fork-time environ; only new exec sessions and child processes
// pick up the new values — unavoidable env-var caveat.
func (c *Client) ApplyContainerEnvDiff(
	ctx context.Context,
	container string,
	set map[string]string,
	unset []string,
) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if strings.TrimSpace(container) == "" {
		return errors.New("container name required")
	}

	for k, v := range set {
		qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
		cur, _ := c.lxc.Run(qctx, "config", "get", container, "environment."+k)
		cancelQ()
		if strings.TrimSpace(cur) == v {
			continue
		}
		sctx, cancelS := context.WithTimeout(ctx, queryTimeout)
		out, err := c.lxc.Run(sctx, "config", "set", container, "environment."+k, v)
		cancelS()
		if err != nil {
			return fmt.Errorf("set environment.%s on %s: %w; output: %s", k, container, err, out)
		}
	}

	for _, k := range unset {
		uctx, cancelU := context.WithTimeout(ctx, queryTimeout)
		out, err := c.lxc.Run(uctx, "config", "unset", container, "environment."+k)
		cancelU()
		if err != nil {
			// `lxc config unset` on a missing key still exits non-zero;
			// treat that as success.
			if strings.Contains(out, "not set") || strings.Contains(out, "doesn't exist") {
				continue
			}
			return fmt.Errorf("unset environment.%s on %s: %w; output: %s", k, container, err, out)
		}
	}
	return nil
}
