package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const queryTimeout = 10 * time.Second

// agentBrowserConfigurator owns the LXD settings required by the browser
// stack. Container-wide resource limits are owned by the resources package
// (the futrx-workspace profile) and are never touched from here.
type agentBrowserConfigurator struct {
	runner command.Runner
}

// EnsureNesting applies the security.nesting config Chrome's sandbox needs.
// The key also lives in the workspace profile; this container-local set
// remains as a belt-and-braces for containers whose profile attach failed.
func (s *Adapter) EnsureNesting(ctx context.Context, containerName string) error {
	return s.config.ensure(ctx, containerName)
}

func (c *agentBrowserConfigurator) ensure(ctx context.Context, containerName string) error {
	if !c.runner.Available() {
		return command.ErrUnavailable
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	current, _ := c.runner.Run(lctx, "config", "get", containerName, "security.nesting")
	if strings.TrimSpace(current) != "true" {
		out, err := c.runner.Run(lctx, "config", "set", containerName, "security.nesting", "true")
		if err != nil {
			return fmt.Errorf("set security.nesting: %w; output: %s", err, out)
		}
	}
	return nil
}
