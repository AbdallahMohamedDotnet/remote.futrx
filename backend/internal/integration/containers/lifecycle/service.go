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

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	hostMappedUID          = 1000000
	containerWorkspacePath = "/workspace"
)

// containerLifecycle owns container state transitions, workspace attachment,
// and launch-time capability orchestration.
type Service struct {
	runtime     *Client
	image       string
	workspace   hostWorkspacePreparer
	resources   ResourceEnsurer
	provisioner LaunchProvisioner
}

// LaunchProvisioner applies best-effort capabilities after a new container is
// created and attached to its workspace.
type LaunchProvisioner interface {
	Provision(ctx context.Context, containerName, displayName string)
}

// ResourceEnsurer converges the shared workspace profile (default resource
// limits) onto a container.
type ResourceEnsurer interface {
	Ensure(ctx context.Context, containerName string) error
}

// NewService returns a container lifecycle service using image for newly
// created project containers.
func NewService(runner command.Runner, image string, resources ResourceEnsurer, provisioner LaunchProvisioner) *Service {
	return &Service{
		runtime:     NewClient(runner),
		image:       image,
		workspace:   hostWorkspacePreparer{uid: hostMappedUID, gid: hostMappedUID},
		resources:   resources,
		provisioner: provisioner,
	}
}

func (l *Service) Launch(ctx context.Context, p serviceproject.Meta) error {
	if !l.runtime.Available() {
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
		// Migration for pre-profile containers: converge the resource
		// envelope on every start of an existing workspace. Best-effort so
		// a missing lxc capability never blocks the container coming up.
		_ = l.resources.Ensure(ctx, p.ContainerName)
		if state == serviceproject.ContainerStateStopped {
			return l.Start(ctx, p.ContainerName)
		}
		return nil
	}

	if err := l.runtime.Launch(ctx, l.image, p.ContainerName); err != nil {
		return err
	}

	// Cap resources before any workload can run in the fresh container.
	_ = l.resources.Ensure(ctx, p.ContainerName)

	if err := l.runtime.AttachDisk(ctx, p.ContainerName, "workspace", p.Cwd, containerWorkspacePath, false); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	// Best-effort: a missing autostart bit or an unavailable capability should
	// not block the container from coming up. These idempotent migrations are
	// retried on the next prompt.
	_ = l.EnsureBootAutostart(ctx, p.ContainerName)
	l.provisioner.Provision(ctx, p.ContainerName, p.Name)

	return nil
}

// EnsureBootAutostart marks the container as auto-starting after a host
// reboot. Without this, a host reboot leaves every project's container in
// `stopped` state until the runner brings it back via a prompt — which
// silently breaks anything time-driven (cron, systemd timers). Idempotent;
// cheap; safe to call on every prompt as a migration for older containers.
func (l *Service) EnsureBootAutostart(ctx context.Context, containerName string) error {
	if !l.runtime.Available() {
		return errors.New("lxc not available")
	}
	return l.runtime.EnsureBootAutostart(ctx, containerName)
}

func (l *Service) Start(ctx context.Context, containerName string) error {
	return l.runtime.Start(ctx, containerName)
}

func (l *Service) Stop(ctx context.Context, containerName string) error {
	return l.runtime.Stop(ctx, containerName)
}

func (l *Service) Delete(ctx context.Context, containerName string) error {
	return l.runtime.Delete(ctx, containerName)
}

func (l *Service) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	return l.runtime.State(ctx, containerName)
}
