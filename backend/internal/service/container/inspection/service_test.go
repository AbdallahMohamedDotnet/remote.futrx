package inspection

import (
	"context"
	"errors"
	"reflect"
	"testing"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

var _ serviceproject.ContainerInspector = (*Service)(nil)

func TestInspectScopesProbesByContainerState(t *testing.T) {
	tests := []struct {
		name       string
		state      serviceproject.ContainerState
		wantEvents []string
	}{
		{
			name:       "missing",
			state:      serviceproject.ContainerStateMissing,
			wantEvents: []string{"state:c1"},
		},
		{
			name:  "stopped",
			state: serviceproject.ContainerStateStopped,
			wantEvents: []string{
				"state:c1",
				"configuration:c1",
				"credentials:c1:STOPPED",
			},
		},
		{
			name:  "frozen",
			state: serviceproject.ContainerStateFrozen,
			wantEvents: []string{
				"state:c1",
				"configuration:c1",
				"credentials:c1:FROZEN",
			},
		},
		{
			name:  "running",
			state: serviceproject.ContainerStateRunning,
			wantEvents: []string{
				"state:c1",
				"configuration:c1",
				"runtime:c1",
				"guest:c1",
				"agents:c1",
				"credentials:c1:RUNNING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			probes := &recordingProbes{events: &events}
			service := NewService(Dependencies{
				States:        recordingStateReader{events: &events, state: tt.state},
				Configuration: probes,
				Runtime:       probes,
				Guest:         probes,
				Agents:        probes,
				Credentials:   probes,
			})

			got, err := service.Inspect(context.Background(), "c1")
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if !reflect.DeepEqual(events, tt.wantEvents) {
				t.Fatalf("events = %v, want %v", events, tt.wantEvents)
			}
			if got.Name != "c1" || got.State != tt.state {
				t.Fatalf("snapshot identity = %#v", got)
			}
			assertStateScopedOutput(t, got, tt.state)
		})
	}
}

func TestInspectReturnsStateErrorBeforeProbing(t *testing.T) {
	wantErr := errors.New("state unavailable")
	events := []string{}
	probes := &recordingProbes{events: &events}
	service := NewService(Dependencies{
		States:        recordingStateReader{events: &events, err: wantErr},
		Configuration: probes,
		Runtime:       probes,
		Guest:         probes,
		Agents:        probes,
		Credentials:   probes,
	})

	got, err := service.Inspect(context.Background(), "c1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Inspect() error = %v, want %v", err, wantErr)
	}
	if want := []string{"state:c1"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if want := (serviceproject.ContainerInspect{Name: "c1"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot = %#v, want name-only snapshot", got)
	}
}

func assertStateScopedOutput(
	t *testing.T,
	got serviceproject.ContainerInspect,
	state serviceproject.ContainerState,
) {
	t.Helper()
	if state == serviceproject.ContainerStateMissing {
		if got.Image != "" || got.PID != 0 || got.OS != nil || got.AuthBundles != nil {
			t.Fatalf("missing-container snapshot contains probed fields: %#v", got)
		}
		return
	}
	if got.Image != "ubuntu" || len(got.AuthBundles) != 1 {
		t.Fatalf("existing-container fields = %#v", got)
	}
	if state != serviceproject.ContainerStateRunning {
		if got.PID != 0 || got.OS != nil || got.Disks != nil || got.Claude.Installed {
			t.Fatalf("non-running snapshot contains live fields: %#v", got)
		}
		return
	}
	if got.PID != 42 || got.OS == nil || got.OS.Hostname != "guest" || len(got.Disks) != 1 || !got.Claude.Installed {
		t.Fatalf("running-container fields = %#v", got)
	}
}

type recordingStateReader struct {
	events *[]string
	state  serviceproject.ContainerState
	err    error
}

func (r recordingStateReader) State(_ context.Context, containerName string) (serviceproject.ContainerState, error) {
	*r.events = append(*r.events, "state:"+containerName)
	return r.state, r.err
}

type recordingProbes struct {
	events *[]string
}

func (p *recordingProbes) InspectConfiguration(
	_ context.Context,
	containerName string,
	out *serviceproject.ContainerInspect,
) {
	*p.events = append(*p.events, "configuration:"+containerName)
	out.Image = "ubuntu"
}

func (p *recordingProbes) InspectRuntime(
	_ context.Context,
	containerName string,
	out *serviceproject.ContainerInspect,
) {
	*p.events = append(*p.events, "runtime:"+containerName)
	out.PID = 42
}

func (p *recordingProbes) InspectGuest(
	_ context.Context,
	containerName string,
) (*serviceproject.OSInfo, []serviceproject.DiskUsage) {
	*p.events = append(*p.events, "guest:"+containerName)
	return &serviceproject.OSInfo{Hostname: "guest"}, []serviceproject.DiskUsage{{MountPath: "/"}}
}

func (p *recordingProbes) InspectAgents(
	_ context.Context,
	containerName string,
) []serviceproject.AgentContainerStatus {
	*p.events = append(*p.events, "agents:"+containerName)
	return []serviceproject.AgentContainerStatus{{ID: "claude", Installed: true}}
}

func (p *recordingProbes) InspectCredentials(
	_ context.Context,
	containerName string,
	state serviceproject.ContainerState,
) []serviceproject.AuthBundleStatus {
	*p.events = append(*p.events, "credentials:"+containerName+":"+string(state))
	return []serviceproject.AuthBundleStatus{{Name: "claude"}}
}
