// Package command defines the transport seam used by container capabilities.
package command

import (
	"context"
	"io"
)

// Runner invokes the underlying container runtime.
type Runner interface {
	Available() bool
	Run(ctx context.Context, args ...string) (string, error)
	RunStdin(ctx context.Context, stdin io.Reader, args ...string) (string, error)
}
