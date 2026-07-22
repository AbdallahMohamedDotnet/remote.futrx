// Package command defines the transport seam used by container capabilities.
package command

import (
	"context"
	"io"
	"time"
)

// Runner invokes the underlying container runtime.
type Runner interface {
	Available() bool
	Run(ctx context.Context, args ...string) (string, error)
	RunStdin(ctx context.Context, stdin io.Reader, args ...string) (string, error)
}

// RunWithTimeout invokes a single r.Run bounded by its own deadline. Callers
// that budget one deadline across several runs derive their own context.
func RunWithTimeout(ctx context.Context, r Runner, timeout time.Duration, args ...string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return r.Run(tctx, args...)
}
