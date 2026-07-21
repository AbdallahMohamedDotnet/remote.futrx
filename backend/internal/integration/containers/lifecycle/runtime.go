package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	launchTimeout = 90 * time.Second
	startTimeout  = 30 * time.Second
	stopTimeout   = 30 * time.Second
	deleteTimeout = 30 * time.Second
	queryTimeout  = 10 * time.Second
)

// Client translates lifecycle operations into LXD CLI calls.
type Client struct {
	runner command.Runner
}

func NewClient(runner command.Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Available() bool {
	return c.runner.Available()
}

func (c *Client) Launch(ctx context.Context, image, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	if out, err := c.runner.Run(lctx, "launch", image, containerName); err != nil {
		return fmt.Errorf("lxc launch: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) AttachDisk(ctx context.Context, container, deviceName, hostSrc, containerPath string, readonly bool) error {
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	args := []string{
		"config", "device", "add", container, deviceName, "disk",
		"source=" + hostSrc,
		"path=" + containerPath,
	}
	if readonly {
		args = append(args, "readonly=true")
	}
	if out, err := c.runner.Run(lctx, args...); err != nil {
		return fmt.Errorf("lxc config device add %s: %w; output: %s", deviceName, err, out)
	}
	return nil
}

func (c *Client) EnsureBootAutostart(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	cur, _ := c.runner.Run(lctx, "config", "get", containerName, "boot.autostart")
	if strings.TrimSpace(cur) == "true" {
		return nil
	}
	if out, err := c.runner.Run(lctx, "config", "set", containerName, "boot.autostart", "true"); err != nil {
		return fmt.Errorf("set boot.autostart: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Start(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if out, err := c.runner.Run(lctx, "start", containerName); err != nil {
		if strings.Contains(out, "is already running") {
			return nil
		}
		return fmt.Errorf("lxc start: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := c.runner.Run(lctx, "stop", containerName); err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "is already stopped") {
			return nil
		}
		return fmt.Errorf("lxc stop: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, containerName string) error {
	if !c.runner.Available() {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if out, err := c.runner.Run(lctx, "delete", "--force", containerName); err != nil {
		if strings.Contains(out, "not found") {
			return nil
		}
		return fmt.Errorf("lxc delete: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	if !c.runner.Available() {
		return serviceproject.ContainerStateUnknown, nil
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.runner.Run(lctx, "info", containerName)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "doesn't exist") {
			return serviceproject.ContainerStateMissing, nil
		}
		return serviceproject.ContainerStateUnknown, fmt.Errorf("lxc info: %w; output: %s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch strings.ToUpper(status) {
			case "RUNNING":
				return serviceproject.ContainerStateRunning, nil
			case "STOPPED":
				return serviceproject.ContainerStateStopped, nil
			case "FROZEN":
				return serviceproject.ContainerStateFrozen, nil
			default:
				return serviceproject.ContainerStateUnknown, nil
			}
		}
	}
	return serviceproject.ContainerStateUnknown, nil
}
