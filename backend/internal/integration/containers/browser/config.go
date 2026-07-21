package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
)

const queryTimeout = 10 * time.Second

// agentBrowserConfigurator owns the LXD settings required by the browser
// stack and removal of its obsolete container-wide resource limits.
type agentBrowserConfigurator struct {
	runner command.Runner
}

// EnsureAgentBrowserLimits applies only container config that Chrome needs and
// removes older browser-induced container-wide CPU/memory limits.
func (s *Service) EnsureLimits(ctx context.Context, containerName string) error {
	return s.config.ensure(ctx, containerName)
}

func (c *agentBrowserConfigurator) ensure(ctx context.Context, containerName string) error {
	if !c.runner.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	current, _ := c.runner.Run(lctx, "config", "get", containerName, "security.nesting")
	if strings.TrimSpace(current) != "true" {
		out, err := c.runner.Run(lctx, "config", "set", containerName, "security.nesting", "true")
		if err != nil {
			cancel()
			return fmt.Errorf("set security.nesting: %w; output: %s", err, out)
		}
	}
	cancel()

	for _, key := range []string{"limits.cpu", "limits.memory"} {
		qctx, qcancel := context.WithTimeout(ctx, queryTimeout)
		current, _ := c.runner.Run(qctx, "config", "get", containerName, key)
		qcancel()
		if strings.TrimSpace(current) == "" {
			continue
		}
		uctx, ucancel := context.WithTimeout(ctx, queryTimeout)
		out, err := c.runner.Run(uctx, "config", "unset", containerName, key)
		ucancel()
		if err != nil {
			return fmt.Errorf("unset %s: %w; output: %s", key, err, out)
		}
	}
	return nil
}
