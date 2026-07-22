package lifecycle

import (
	"context"
	"errors"
	"slices"
	"testing"

	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

type recordingRuntime struct {
	events       *[]string
	available    bool
	state        serviceproject.ContainerState
	stateErr     error
	launchErr    error
	attachErr    error
	autostartErr error
	startErr     error
	deleteErr    error
}

func (r recordingRuntime) Available() bool {
	*r.events = append(*r.events, "runtime available")
	return r.available
}

func (r recordingRuntime) Launch(_ context.Context, image, containerName string) error {
	*r.events = append(*r.events, "runtime launch "+image+" "+containerName)
	return r.launchErr
}

func (r recordingRuntime) AttachDisk(
	_ context.Context,
	container,
	deviceName,
	hostSource,
	containerPath string,
	readonly bool,
) error {
	mode := "read-write"
	if readonly {
		mode = "read-only"
	}
	*r.events = append(*r.events, "runtime attach "+container+" "+deviceName+" "+hostSource+" "+containerPath+" "+mode)
	return r.attachErr
}

func (r recordingRuntime) EnsureBootAutostart(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "runtime autostart "+containerName)
	return r.autostartErr
}

func (r recordingRuntime) Start(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "runtime start "+containerName)
	return r.startErr
}

func (r recordingRuntime) Stop(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "runtime stop "+containerName)
	return nil
}

func (r recordingRuntime) Restart(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "runtime restart "+containerName)
	return nil
}

func (r recordingRuntime) Delete(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "runtime delete "+containerName)
	return r.deleteErr
}

func (r recordingRuntime) State(_ context.Context, containerName string) (serviceproject.ContainerState, error) {
	*r.events = append(*r.events, "runtime state "+containerName)
	return r.state, r.stateErr
}

type recordingWorkspace struct {
	events *[]string
	err    error
}

func (w recordingWorkspace) Prepare(path string) error {
	*w.events = append(*w.events, "workspace prepare "+path)
	return w.err
}

type recordingResources struct {
	events *[]string
	err    error
}

func (r recordingResources) Ensure(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "resources ensure "+containerName)
	return r.err
}

func (r recordingResources) SetLimits(_ context.Context, containerName, cpu, memory, disk string) error {
	*r.events = append(*r.events, "resources limits "+containerName+" "+cpu+" "+memory+" "+disk)
	return r.err
}

type recordingProvisioner struct{ events *[]string }

func (p recordingProvisioner) Provision(_ context.Context, containerName, displayName string) {
	*p.events = append(*p.events, "provision "+containerName+" "+displayName)
}

func TestLaunchNewContainerKeepsApplicationOrderAndBestEffortResources(t *testing.T) {
	var events []string
	runtime := recordingRuntime{
		events:       &events,
		available:    true,
		state:        serviceproject.ContainerStateMissing,
		autostartErr: errors.New("autostart migration failed"),
	}
	service := NewService(
		runtime,
		"local:remote-base",
		recordingWorkspace{events: &events},
		recordingResources{events: &events, err: errors.New("resource migration failed")},
		recordingProvisioner{events: &events},
	)

	err := service.Launch(context.Background(), serviceproject.Meta{
		Name:          "My Project",
		Cwd:           "/host/workspaces/project-1",
		ContainerName: "project-1",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	want := []string{
		"runtime available",
		"workspace prepare /host/workspaces/project-1",
		"runtime state project-1",
		"runtime launch local:remote-base project-1",
		"resources ensure project-1",
		"runtime attach project-1 workspace /host/workspaces/project-1 /workspace read-write",
		"runtime available",
		"runtime autostart project-1",
		"provision project-1 My Project",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events:\n got: %q\nwant: %q", events, want)
	}
}

func TestLaunchExistingContainerEnsuresResourcesBeforeStart(t *testing.T) {
	tests := []struct {
		name  string
		state serviceproject.ContainerState
		want  []string
	}{
		{
			name:  "running",
			state: serviceproject.ContainerStateRunning,
			want: []string{
				"runtime available",
				"workspace prepare /host/workspaces/project-1",
				"runtime state project-1",
				"resources ensure project-1",
			},
		},
		{
			name:  "stopped",
			state: serviceproject.ContainerStateStopped,
			want: []string{
				"runtime available",
				"workspace prepare /host/workspaces/project-1",
				"runtime state project-1",
				"resources ensure project-1",
				"runtime start project-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []string
			service := NewService(
				recordingRuntime{events: &events, available: true, state: tt.state},
				"unused",
				recordingWorkspace{events: &events},
				recordingResources{events: &events, err: errors.New("resource migration failed")},
				recordingProvisioner{events: &events},
			)

			err := service.Launch(context.Background(), serviceproject.Meta{
				Cwd:           "/host/workspaces/project-1",
				ContainerName: "project-1",
			})
			if err != nil {
				t.Fatalf("Launch: %v", err)
			}
			if !slices.Equal(events, tt.want) {
				t.Fatalf("events:\n got: %q\nwant: %q", events, tt.want)
			}
		})
	}
}

func TestLaunchAppliesProjectResourceLimitsBeforeStarting(t *testing.T) {
	var events []string
	service := NewService(
		recordingRuntime{events: &events, available: true, state: serviceproject.ContainerStateStopped},
		"unused",
		recordingWorkspace{events: &events},
		recordingResources{events: &events},
		recordingProvisioner{events: &events},
	)

	err := service.Launch(context.Background(), serviceproject.Meta{
		Cwd:           "/host/workspaces/project-1",
		ContainerName: "project-1",
		ResourceLimits: &serviceproject.ContainerLimits{
			CPU: "4", Memory: "8GiB", Disk: "40GiB",
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	want := []string{
		"runtime available",
		"workspace prepare /host/workspaces/project-1",
		"runtime state project-1",
		"resources ensure project-1",
		"resources limits project-1 4 8GiB 40GiB",
		"runtime start project-1",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events:\n got: %q\nwant: %q", events, want)
	}
}

func TestLaunchRollsBackNewContainerWhenWorkspaceAttachmentFails(t *testing.T) {
	var events []string
	wantErr := errors.New("attach failed")
	service := NewService(
		recordingRuntime{
			events:    &events,
			available: true,
			state:     serviceproject.ContainerStateMissing,
			attachErr: wantErr,
		},
		"local:remote-base",
		recordingWorkspace{events: &events},
		recordingResources{events: &events},
		recordingProvisioner{events: &events},
	)

	err := service.Launch(context.Background(), serviceproject.Meta{
		Name:          "My Project",
		Cwd:           "/host/workspaces/project-1",
		ContainerName: "project-1",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Launch() error = %v, want %v", err, wantErr)
	}

	want := []string{
		"runtime available",
		"workspace prepare /host/workspaces/project-1",
		"runtime state project-1",
		"runtime launch local:remote-base project-1",
		"resources ensure project-1",
		"runtime attach project-1 workspace /host/workspaces/project-1 /workspace read-write",
		"runtime delete project-1",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events:\n got: %q\nwant: %q", events, want)
	}
}

func TestLaunchCleansUpAfterFailedRuntimeLaunch(t *testing.T) {
	var events []string
	wantErr := errors.New("launch failed after creating instance")
	service := NewService(
		recordingRuntime{
			events:    &events,
			available: true,
			state:     serviceproject.ContainerStateMissing,
			launchErr: wantErr,
		},
		"local:remote-base",
		recordingWorkspace{events: &events},
		recordingResources{events: &events},
		recordingProvisioner{events: &events},
	)

	err := service.Launch(context.Background(), serviceproject.Meta{
		Cwd:           "/host/workspaces/project-1",
		ContainerName: "project-1",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Launch() error = %v, want %v", err, wantErr)
	}
	want := []string{
		"runtime available",
		"workspace prepare /host/workspaces/project-1",
		"runtime state project-1",
		"runtime launch local:remote-base project-1",
		"runtime delete project-1",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events:\n got: %q\nwant: %q", events, want)
	}
}

func TestLaunchReportsProvisioningAndRollbackFailures(t *testing.T) {
	var events []string
	attachErr := errors.New("attach failed")
	deleteErr := errors.New("delete failed")
	service := NewService(
		recordingRuntime{
			events:    &events,
			available: true,
			state:     serviceproject.ContainerStateMissing,
			attachErr: attachErr,
			deleteErr: deleteErr,
		},
		"local:remote-base",
		recordingWorkspace{events: &events},
		recordingResources{events: &events},
		recordingProvisioner{events: &events},
	)

	err := service.Launch(context.Background(), serviceproject.Meta{
		Cwd:           "/host/workspaces/project-1",
		ContainerName: "project-1",
	})
	if !errors.Is(err, attachErr) || !errors.Is(err, deleteErr) {
		t.Fatalf("Launch() error = %v, want joined attach and rollback failures", err)
	}
}
