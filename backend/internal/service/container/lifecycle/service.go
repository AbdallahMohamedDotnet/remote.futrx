// Package lifecycle owns project-container state transitions and launch-time
// orchestration without depending on LXD or host-filesystem details.
package lifecycle

import (
	"context"
	"errors"
	"fmt"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const containerWorkspacePath = "/workspace"

type Runtime interface {
	Available() bool
	Launch(ctx context.Context, image, containerName string) error
	AttachDisk(ctx context.Context, container, deviceName, hostSource, containerPath string, readonly bool) error
	EnsureBootAutostart(ctx context.Context, containerName string) error
	Start(ctx context.Context, containerName string) error
	Stop(ctx context.Context, containerName string) error
	Restart(ctx context.Context, containerName string) error
	Delete(ctx context.Context, containerName string) error
	State(ctx context.Context, containerName string) (serviceproject.ContainerState, error)
}

type WorkspacePreparer interface {
	Prepare(path string) error
}

type ResourceEnsurer interface {
	Ensure(ctx context.Context, containerName string) error
}

type LaunchProvisioner interface {
	Provision(ctx context.Context, containerName, displayName string)
}

type Service struct {
	runtime     Runtime
	image       string
	workspace   WorkspacePreparer
	resources   ResourceEnsurer
	provisioner LaunchProvisioner
}

func NewService(
	runtime Runtime,
	image string,
	workspace WorkspacePreparer,
	resources ResourceEnsurer,
	provisioner LaunchProvisioner,
) *Service {
	return &Service{
		runtime:     runtime,
		image:       image,
		workspace:   workspace,
		resources:   resources,
		provisioner: provisioner,
	}
}

func (s *Service) Available() bool {
	return s.runtime.Available()
}

func (s *Service) Launch(ctx context.Context, project serviceproject.Meta) error {
	if !s.runtime.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}

	if err := s.workspace.Prepare(project.Cwd); err != nil {
		return err
	}

	state, err := s.runtime.State(ctx, project.ContainerName)
	if err != nil {
		return err
	}
	if state != serviceproject.ContainerStateMissing {
		_ = s.resources.Ensure(ctx, project.ContainerName)
		if state == serviceproject.ContainerStateStopped {
			return s.runtime.Start(ctx, project.ContainerName)
		}
		return nil
	}

	if err := s.runtime.Launch(ctx, s.image, project.ContainerName); err != nil {
		return err
	}

	_ = s.resources.Ensure(ctx, project.ContainerName)

	if err := s.runtime.AttachDisk(
		ctx,
		project.ContainerName,
		"workspace",
		project.Cwd,
		containerWorkspacePath,
		false,
	); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	_ = s.EnsureBootAutostart(ctx, project.ContainerName)
	s.provisioner.Provision(ctx, project.ContainerName, project.Name)
	return nil
}

func (s *Service) EnsureBootAutostart(ctx context.Context, containerName string) error {
	if !s.runtime.Available() {
		return errors.New("lxc not available")
	}
	return s.runtime.EnsureBootAutostart(ctx, containerName)
}

// EnsureResources converges the fleet-default resource envelope (managed
// LXD profile) onto a container. Exposed so the project service's startup
// reconcile can converge the whole fleet — every deploy restarts the
// backend, so `update.sh` alone brings any box's containers to the pins.
func (s *Service) EnsureResources(ctx context.Context, containerName string) error {
	if !s.runtime.Available() {
		return errors.New("lxc not available")
	}
	return s.resources.Ensure(ctx, containerName)
}

func (s *Service) Start(ctx context.Context, containerName string) error {
	// Converge the resource envelope on every explicit start, not only in
	// Launch: the project service short-circuits to Start for containers
	// that already exist. Best-effort — a profile hiccup must not block
	// the start; the startup reconcile sweep retries and logs.
	_ = s.resources.Ensure(ctx, containerName)
	return s.runtime.Start(ctx, containerName)
}

func (s *Service) Stop(ctx context.Context, containerName string) error {
	return s.runtime.Stop(ctx, containerName)
}

// Restart force-restarts a container from the host — the regain-control
// path for a workspace wedged at its resource limits (a host-kernel kill
// needs no cooperation from processes inside). Converges the resource
// envelope after the fresh boot.
func (s *Service) Restart(ctx context.Context, containerName string) error {
	if !s.runtime.Available() {
		return errors.New("lxc not available")
	}
	if err := s.runtime.Restart(ctx, containerName); err != nil {
		return err
	}
	_ = s.resources.Ensure(ctx, containerName)
	return nil
}

func (s *Service) Delete(ctx context.Context, containerName string) error {
	return s.runtime.Delete(ctx, containerName)
}

func (s *Service) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	return s.runtime.State(ctx, containerName)
}
