package launch

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type callRecorder struct{ calls []string }

type failingCredentials struct{ recorder *callRecorder }

func (f failingCredentials) EnsureRegistered(_ context.Context, containerName string) error {
	f.recorder.calls = append(f.recorder.calls, "credentials "+containerName)
	return errors.New("credentials failed")
}

type failingWorkspace struct{ recorder *callRecorder }

func (f failingWorkspace) EnsureSkillLinks(_ context.Context, containerName string) error {
	f.recorder.calls = append(f.recorder.calls, "workspace "+containerName)
	return errors.New("workspace failed")
}

type failingBrowser struct{ recorder *callRecorder }

func (f failingBrowser) EnsureScript(_ context.Context, containerName string) error {
	f.recorder.calls = append(f.recorder.calls, "browser script "+containerName)
	return errors.New("browser script failed")
}

func (f failingBrowser) EnsureSkill(_ context.Context, containerName string) error {
	f.recorder.calls = append(f.recorder.calls, "browser skill "+containerName)
	return errors.New("browser skill failed")
}

func (f failingBrowser) EnsureLimits(_ context.Context, containerName string) error {
	f.recorder.calls = append(f.recorder.calls, "browser limits "+containerName)
	return errors.New("browser limits failed")
}

type failingCodeServer struct{ recorder *callRecorder }

func (f failingCodeServer) Ensure(_ context.Context, containerName, displayName string) error {
	f.recorder.calls = append(f.recorder.calls, "code-server "+containerName+" "+displayName)
	return errors.New("code-server failed")
}

func TestProvisionKeepsBestEffortCapabilityOrder(t *testing.T) {
	recorder := &callRecorder{}
	provisioner := NewProvisioner(
		failingCredentials{recorder: recorder},
		failingWorkspace{recorder: recorder},
		failingBrowser{recorder: recorder},
		failingCodeServer{recorder: recorder},
	)

	provisioner.Provision(context.Background(), "project-1", "My Project")

	want := []string{
		"credentials project-1",
		"workspace project-1",
		"browser script project-1",
		"browser skill project-1",
		"browser limits project-1",
		"code-server project-1 My Project",
	}
	if !slices.Equal(recorder.calls, want) {
		t.Fatalf("calls: got %q, want %q", recorder.calls, want)
	}
}
