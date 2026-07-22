package inspection

import (
	"context"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

// quickCommandRunner applies the short diagnostic timeout shared by every
// best-effort inspection source.
type quickCommandRunner struct {
	runner  command.Runner
	timeout time.Duration
}

func (r *quickCommandRunner) run(parent context.Context, args ...string) (string, error) {
	return command.RunWithTimeout(parent, r.runner, r.timeout, args...)
}
