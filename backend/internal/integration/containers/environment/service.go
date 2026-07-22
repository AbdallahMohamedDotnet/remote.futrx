// Package environment applies project environment-variable configuration to
// containers.
package environment

// ApplyContainerEnvDiff drives LXD's built-in environment.* config keys.
// Used by the project-secrets flow to push per-project secrets into the
// project's container, so any subsequent `lxc exec` (agent prompt or
// interactive shell) inherits the var automatically.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const queryTimeout = 10 * time.Second

// Service owns container environment configuration.
type Service struct {
	runner command.Runner
}

// NewService returns an environment service backed by runner.
func NewService(runner command.Runner) *Service {
	return &Service{runner: runner}
}

// ApplyContainerEnvDiff sets the values in set and removes the keys in unset.
// Idempotent: a set that already matches the current value is a no-op (cheap
// read before write). Returns the first error encountered.
//
// Either map can be nil/empty. Already-running processes inside the container
// keep their fork-time environ; only new exec sessions and child processes
// pick up the new values — unavoidable env-var caveat.
func (s *Service) ApplyDiff(
	ctx context.Context,
	container string,
	set map[string]string,
	unset []string,
) error {
	if !s.runner.Available() {
		return errors.New("lxc not available")
	}
	if strings.TrimSpace(container) == "" {
		return errors.New("container name required")
	}

	for k, v := range set {
		qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
		cur, _ := s.runner.Run(qctx, "config", "get", container, "environment."+k)
		cancelQ()
		if strings.TrimSpace(cur) == v {
			continue
		}
		sctx, cancelS := context.WithTimeout(ctx, queryTimeout)
		// End flag parsing before the value. PEM/OpenSSH secrets commonly start
		// with "-----BEGIN", which LXC otherwise interprets as a CLI flag.
		_, err := s.runner.Run(sctx, "config", "set", container, "environment."+k, "--", v)
		cancelS()
		if err != nil {
			// LXC can echo an invalid argument in its output. Never attach that
			// output here because the argument is the secret value itself.
			return fmt.Errorf("set environment.%s on %s: %w", k, container, err)
		}
	}

	for _, k := range unset {
		uctx, cancelU := context.WithTimeout(ctx, queryTimeout)
		out, err := s.runner.Run(uctx, "config", "unset", container, "environment."+k)
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
