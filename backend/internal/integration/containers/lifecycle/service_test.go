package lifecycle

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type runnerResponse struct {
	out string
	err error
}

type recordingRunner struct {
	available bool
	responses map[string]runnerResponse
	events    *[]string
}

func (r *recordingRunner) Available() bool { return r.available }

func (r *recordingRunner) Run(_ context.Context, args ...string) (string, error) {
	call := strings.Join(args, " ")
	*r.events = append(*r.events, "lxc "+call)
	response := r.responses[call]
	return response.out, response.err
}

func (r *recordingRunner) RunStdin(ctx context.Context, _ io.Reader, args ...string) (string, error) {
	return r.Run(ctx, args...)
}

type recordingResources struct {
	events *[]string
	err    error
}

func (r recordingResources) Ensure(_ context.Context, containerName string) error {
	*r.events = append(*r.events, "resources "+containerName)
	return r.err
}

type recordingProvisioner struct{ events *[]string }

func (p recordingProvisioner) Provision(_ context.Context, containerName, displayName string) {
	*p.events = append(*p.events, "provision "+containerName+" "+displayName)
}

func TestLaunchNewContainerKeepsCommandAndCapabilityOrder(t *testing.T) {
	var events []string
	runner := &recordingRunner{
		available: true,
		responses: map[string]runnerResponse{
			"info project-1": {out: "Error: Instance not found", err: errors.New("exit 1")},
		},
		events: &events,
	}
	resources := recordingResources{events: &events, err: errors.New("resource migration failed")}
	service := NewService(runner, "local:remote-base", resources, recordingProvisioner{events: &events})
	service.workspace = hostWorkspacePreparer{uid: os.Getuid(), gid: os.Getgid()}
	cwd := t.TempDir()

	err := service.Launch(context.Background(), serviceproject.Meta{
		Name:          "My Project",
		Cwd:           cwd,
		ContainerName: "project-1",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	want := []string{
		"lxc info project-1",
		"lxc launch local:remote-base project-1",
		"resources project-1",
		"lxc config device add project-1 workspace disk source=" + cwd + " path=/workspace",
		"lxc config get project-1 boot.autostart",
		"lxc config set project-1 boot.autostart true",
		"provision project-1 My Project",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events:\n got: %q\nwant: %q", events, want)
	}
}

func TestLaunchExistingContainerEnsuresResourcesBeforeStarting(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   []string
	}{
		{
			name:   "running",
			status: "RUNNING",
			want:   []string{"lxc info project-1", "resources project-1"},
		},
		{
			name:   "stopped",
			status: "STOPPED",
			want:   []string{"lxc info project-1", "resources project-1", "lxc start project-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []string
			runner := &recordingRunner{
				available: true,
				responses: map[string]runnerResponse{
					"info project-1": {out: "Status: " + tt.status},
				},
				events: &events,
			}
			resources := recordingResources{events: &events, err: errors.New("resource migration failed")}
			service := NewService(runner, "unused", resources, recordingProvisioner{events: &events})
			service.workspace = hostWorkspacePreparer{uid: os.Getuid(), gid: os.Getgid()}

			if err := service.Launch(context.Background(), serviceproject.Meta{
				Cwd:           t.TempDir(),
				ContainerName: "project-1",
			}); err != nil {
				t.Fatalf("Launch: %v", err)
			}
			if !slices.Equal(events, tt.want) {
				t.Fatalf("events: got %q, want %q", events, tt.want)
			}
		})
	}
}

func TestStateMapsLXCOutput(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		out       string
		err       error
		want      serviceproject.ContainerState
		wantErr   bool
	}{
		{name: "unavailable", want: serviceproject.ContainerStateUnknown},
		{name: "running", available: true, out: "Name: c1\nStatus: RUNNING\n", want: serviceproject.ContainerStateRunning},
		{name: "stopped", available: true, out: "Status: stopped", want: serviceproject.ContainerStateStopped},
		{name: "frozen", available: true, out: "Status: Frozen", want: serviceproject.ContainerStateFrozen},
		{name: "unrecognized", available: true, out: "Status: EVACUATED", want: serviceproject.ContainerStateUnknown},
		{name: "missing status", available: true, out: "Name: c1", want: serviceproject.ContainerStateUnknown},
		{name: "not found", available: true, out: "Error: Instance not found", err: errors.New("exit 1"), want: serviceproject.ContainerStateMissing},
		{name: "does not exist", available: true, out: "Instance doesn't exist", err: errors.New("exit 1"), want: serviceproject.ContainerStateMissing},
		{name: "runtime error", available: true, out: "daemon unavailable", err: errors.New("exit 1"), want: serviceproject.ContainerStateUnknown, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []string
			runner := &recordingRunner{
				available: tt.available,
				responses: map[string]runnerResponse{
					"info c1": {out: tt.out, err: tt.err},
				},
				events: &events,
			}
			service := NewService(runner, "unused", recordingResources{events: &events}, recordingProvisioner{events: &events})

			got, err := service.State(context.Background(), "c1")
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("State() = (%q, %v), want (%q, error=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
