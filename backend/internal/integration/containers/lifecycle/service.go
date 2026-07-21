// Package lifecycle owns project-container state transitions and workspace
// attachment.
package lifecycle

// Container lifecycle: Launch / Start / Stop / Delete / State plus the
// one-time boot configuration. Provider-agnostic credential seeding is
// dispatched through the configured agent profiles.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	hostMappedUID          = 1000000
	containerWorkspacePath = "/workspace"
	launchTimeout          = 90 * time.Second
	startTimeout           = 30 * time.Second
	stopTimeout            = 30 * time.Second
	deleteTimeout          = 30 * time.Second
	queryTimeout           = 10 * time.Second
)

// containerLifecycle owns container state transitions, workspace attachment,
// and launch-time capability orchestration.
type Service struct {
	runner      command.Runner
	image       string
	workspace   hostWorkspacePreparer
	provisioner launchProvisioner
}

type launchProvisioner interface {
	Provision(ctx context.Context, containerName, displayName string)
}

// NewService returns a container lifecycle service using image for newly
// created project containers.
func NewService(runner command.Runner, image string, provisioner launchProvisioner) *Service {
	return &Service{
		runner:      runner,
		image:       image,
		workspace:   hostWorkspacePreparer{uid: hostMappedUID, gid: hostMappedUID},
		provisioner: provisioner,
	}
}

func (l *Service) Launch(ctx context.Context, p serviceproject.Meta) error {
	if !l.runner.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}

	if err := l.workspace.prepare(p.Cwd); err != nil {
		return err
	}

	state, err := l.State(ctx, p.ContainerName)
	if err != nil {
		return err
	}
	if state != serviceproject.ContainerStateMissing {
		if state == serviceproject.ContainerStateStopped {
			return l.Start(ctx, p.ContainerName)
		}
		return nil
	}

	lctx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	if out, err := l.runner.Run(lctx, "launch", l.image, p.ContainerName); err != nil {
		return fmt.Errorf("lxc launch: %w; output: %s", err, out)
	}

	if err := l.attachDisk(ctx, p.ContainerName, "workspace", p.Cwd, containerWorkspacePath, false); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	// Best-effort: a missing autostart bit or an unavailable capability should
	// not block the container from coming up. These idempotent migrations are
	// retried on the next prompt.
	_ = l.EnsureBootAutostart(ctx, p.ContainerName)
	l.provisioner.Provision(ctx, p.ContainerName, p.Name)

	return nil
}

func (l *Service) attachDisk(ctx context.Context, container, deviceName, hostSrc, containerPath string, readonly bool) error {
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
	if out, err := l.runner.Run(lctx, args...); err != nil {
		return fmt.Errorf("lxc config device add %s: %w; output: %s", deviceName, err, out)
	}
	return nil
}

// EnsureBootAutostart marks the container as auto-starting after a host
// reboot. Without this, a host reboot leaves every project's container in
// `stopped` state until the runner brings it back via a prompt — which
// silently breaks anything time-driven (cron, systemd timers). Idempotent;
// cheap; safe to call on every prompt as a migration for older containers.
func (l *Service) EnsureBootAutostart(ctx context.Context, containerName string) error {
	if !l.runner.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	cur, _ := l.runner.Run(lctx, "config", "get", containerName, "boot.autostart")
	if strings.TrimSpace(cur) == "true" {
		return nil
	}
	if out, err := l.runner.Run(lctx, "config", "set", containerName, "boot.autostart", "true"); err != nil {
		return fmt.Errorf("set boot.autostart: %w; output: %s", err, out)
	}
	return nil
}

func (l *Service) Start(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if out, err := l.runner.Run(lctx, "start", containerName); err != nil {
		if strings.Contains(out, "is already running") {
			return nil
		}
		return fmt.Errorf("lxc start: %w; output: %s", err, out)
	}
	return nil
}

func (l *Service) Stop(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := l.runner.Run(lctx, "stop", containerName); err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "is already stopped") {
			return nil
		}
		return fmt.Errorf("lxc stop: %w; output: %s", err, out)
	}
	return nil
}

func (l *Service) Delete(ctx context.Context, containerName string) error {
	if !l.runner.Available() {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if out, err := l.runner.Run(lctx, "delete", "--force", containerName); err != nil {
		if strings.Contains(out, "not found") {
			return nil
		}
		return fmt.Errorf("lxc delete: %w; output: %s", err, out)
	}
	return nil
}

func (l *Service) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	if !l.runner.Available() {
		return serviceproject.ContainerStateUnknown, nil
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := l.runner.Run(lctx, "info", containerName)
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
