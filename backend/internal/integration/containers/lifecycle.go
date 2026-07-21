package containers

// Container lifecycle: Launch / Start / Stop / Delete / State plus the
// one-time boot configuration. Provider-agnostic credential seeding is
// dispatched through the configured agent profiles.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// containerLifecycle owns container state transitions, workspace attachment,
// and launch-time capability orchestration.
type containerLifecycle struct {
	lxc         CommandRunner
	image       string
	workspace   hostWorkspacePreparer
	provisioner *containerLaunchProvisioner
}

func (c *Client) Launch(ctx context.Context, p serviceproject.Meta) error {
	return c.lifecycle.launch(ctx, p)
}

func (l *containerLifecycle) launch(ctx context.Context, p serviceproject.Meta) error {
	if !l.lxc.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}

	if err := l.workspace.prepare(p.Cwd); err != nil {
		return err
	}

	state, err := l.state(ctx, p.ContainerName)
	if err != nil {
		return err
	}
	if state != serviceproject.ContainerStateMissing {
		if state == serviceproject.ContainerStateStopped {
			return l.start(ctx, p.ContainerName)
		}
		return nil
	}

	lctx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	if out, err := l.lxc.Run(lctx, "launch", l.image, p.ContainerName); err != nil {
		return fmt.Errorf("lxc launch: %w; output: %s", err, out)
	}

	if err := l.attachDisk(ctx, p.ContainerName, "workspace", p.Cwd, containerWorkspacePath, false); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	// Best-effort: a missing autostart bit or an unavailable capability should
	// not block the container from coming up. These idempotent migrations are
	// retried on the next prompt.
	_ = l.ensureBootAutostart(ctx, p.ContainerName)
	l.provisioner.provision(ctx, p.ContainerName, p.Name)

	return nil
}

func (l *containerLifecycle) attachDisk(ctx context.Context, container, deviceName, hostSrc, containerPath string, readonly bool) error {
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
	if out, err := l.lxc.Run(lctx, args...); err != nil {
		return fmt.Errorf("lxc config device add %s: %w; output: %s", deviceName, err, out)
	}
	return nil
}

// EnsureBootAutostart marks the container as auto-starting after a host
// reboot. Without this, a host reboot leaves every project's container in
// `stopped` state until the runner brings it back via a prompt — which
// silently breaks anything time-driven (cron, systemd timers). Idempotent;
// cheap; safe to call on every prompt as a migration for older containers.
func (c *Client) EnsureBootAutostart(ctx context.Context, containerName string) error {
	return c.lifecycle.ensureBootAutostart(ctx, containerName)
}

func (l *containerLifecycle) ensureBootAutostart(ctx context.Context, containerName string) error {
	if !l.lxc.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	cur, _ := l.lxc.Run(lctx, "config", "get", containerName, "boot.autostart")
	if strings.TrimSpace(cur) == "true" {
		return nil
	}
	if out, err := l.lxc.Run(lctx, "config", "set", containerName, "boot.autostart", "true"); err != nil {
		return fmt.Errorf("set boot.autostart: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Start(ctx context.Context, containerName string) error {
	return c.lifecycle.start(ctx, containerName)
}

func (l *containerLifecycle) start(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if out, err := l.lxc.Run(lctx, "start", containerName); err != nil {
		if strings.Contains(out, "is already running") {
			return nil
		}
		return fmt.Errorf("lxc start: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, containerName string) error {
	return c.lifecycle.stop(ctx, containerName)
}

func (l *containerLifecycle) stop(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := l.lxc.Run(lctx, "stop", containerName); err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "is already stopped") {
			return nil
		}
		return fmt.Errorf("lxc stop: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, containerName string) error {
	return c.lifecycle.delete(ctx, containerName)
}

func (l *containerLifecycle) delete(ctx context.Context, containerName string) error {
	if !l.lxc.Available() {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if out, err := l.lxc.Run(lctx, "delete", "--force", containerName); err != nil {
		if strings.Contains(out, "not found") {
			return nil
		}
		return fmt.Errorf("lxc delete: %w; output: %s", err, out)
	}
	return nil
}

func (c *Client) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	return c.lifecycle.state(ctx, containerName)
}

func (l *containerLifecycle) state(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	if !l.lxc.Available() {
		return serviceproject.ContainerStateUnknown, nil
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := l.lxc.Run(lctx, "info", containerName)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "doesn't exist") {
			return serviceproject.ContainerStateMissing, nil
		}
		return serviceproject.ContainerStateUnknown, fmt.Errorf("lxc info: %w; output: %s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Status:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch strings.ToUpper(s) {
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
