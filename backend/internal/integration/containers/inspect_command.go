package containers

import (
	"context"
	"time"
)

// quickCommandRunner applies the short diagnostic timeout shared by every
// best-effort inspection source.
type quickCommandRunner struct {
	lxc     CommandRunner
	timeout time.Duration
}

func (r *quickCommandRunner) run(parent context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()
	return r.lxc.Run(ctx, args...)
}
