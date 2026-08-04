package image

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

type runtimeResponse struct {
	output string
	err    error
}

type recordingRuntime struct {
	available       bool
	events          []string
	launchResponse  runtimeResponse
	scriptResponses []runtimeResponse
	stopResponse    runtimeResponse
	publishResponse runtimeResponse
}

func (r *recordingRuntime) Available() bool {
	r.events = append(r.events, "available")
	return r.available
}

func (r *recordingRuntime) DeleteContainer(_ context.Context, containerName string) (string, error) {
	r.events = append(r.events, "delete "+containerName)
	return "", nil
}

func (r *recordingRuntime) LaunchContainer(_ context.Context, sourceImage, containerName string) (string, error) {
	r.events = append(r.events, "launch "+sourceImage+" "+containerName)
	return r.launchResponse.output, r.launchResponse.err
}

func (r *recordingRuntime) ExecuteScript(_ context.Context, containerName, script string) (string, error) {
	responseIndex := 0
	for _, event := range r.events {
		if strings.HasPrefix(event, "script ") {
			responseIndex++
		}
	}
	r.events = append(r.events, "script "+containerName+" "+script)
	if responseIndex >= len(r.scriptResponses) {
		return "", nil
	}
	response := r.scriptResponses[responseIndex]
	return response.output, response.err
}

func (r *recordingRuntime) StopContainer(_ context.Context, containerName string) (string, error) {
	r.events = append(r.events, "stop "+containerName)
	return r.stopResponse.output, r.stopResponse.err
}

func (r *recordingRuntime) PublishImage(_ context.Context, containerName, alias, description string) (string, error) {
	r.events = append(r.events, "publish "+containerName+" "+alias+" "+description)
	return r.publishResponse.output, r.publishResponse.err
}

type recordingProfileSource struct {
	profiles  []provisioning.Profile
	snapshots int
}

func (s *recordingProfileSource) Snapshot() []provisioning.Profile {
	s.snapshots++
	return append([]provisioning.Profile(nil), s.profiles...)
}

func configuredProfiles() []provisioning.Profile {
	return []provisioning.Profile{{
		ID: "alpha",
		CLI: provisioning.CLISpec{
			ImageLabel:  "alpha-cli",
			Binary:      "alpha",
			PackageName: "@example/alpha",
			Version:     "1.2.3",
		},
	}}
}

func TestBuildPreservesImageWorkflowOrder(t *testing.T) {
	runtime := &recordingRuntime{available: true}
	profiles := &recordingProfileSource{profiles: configuredProfiles()}
	builder := NewBuilder(runtime, profiles, "browser-install", []byte("code-server-install"), nil)
	builder.networkWarmup = 0

	if err := builder.Build(context.Background(), ""); err != nil {
		t.Fatalf("Build: %v", err)
	}
	installScript, err := InstallScript(profiles.profiles)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"available",
		"delete " + baseImageBuilderName,
		"launch " + SourceImage + " " + baseImageBuilderName,
		"script " + baseImageBuilderName + " " + installScript,
		"script " + baseImageBuilderName + " browser-install",
		"script " + baseImageBuilderName + " code-server-install",
		"stop " + baseImageBuilderName,
		"publish " + baseImageBuilderName + " " + Alias + " futrx remote dev base: ubuntu 24.04 + node 22 + alpha-cli",
		"delete " + baseImageBuilderName,
	}
	assertEvents(t, runtime.events, want)
	if profiles.snapshots != 1 {
		t.Fatalf("profile snapshots = %d, want 1", profiles.snapshots)
	}
}

func TestBuildPreservesErrorOutputAndDeferredCleanup(t *testing.T) {
	runtime := &recordingRuntime{
		available: true,
		scriptResponses: []runtimeResponse{
			{},
			{output: strings.Repeat("x", 2001), err: errors.New("exit 1")},
		},
	}
	builder := NewBuilder(
		runtime,
		&recordingProfileSource{profiles: configuredProfiles()},
		"browser-install",
		[]byte("code-server-install"),
		nil,
	)
	builder.networkWarmup = 0

	err := builder.Build(context.Background(), "custom-alias")
	wantErr := "agent browser install script: exit 1; output: ..." + strings.Repeat("x", 2000)
	if err == nil || err.Error() != wantErr {
		t.Fatalf("Build error = %v, want %q", err, wantErr)
	}
	if len(runtime.events) < 2 || runtime.events[len(runtime.events)-1] != "delete "+baseImageBuilderName {
		t.Fatalf("events = %q; deferred cleanup did not run", runtime.events)
	}
}

func TestBuildUnavailablePreservesErrorAndDoesNotMutateRuntime(t *testing.T) {
	runtime := &recordingRuntime{}
	builder := NewBuilder(
		runtime,
		&recordingProfileSource{profiles: configuredProfiles()},
		"browser-install",
		[]byte("code-server-install"),
		nil,
	)

	err := builder.Build(context.Background(), "custom-alias")
	const want = "lxc CLI not found on PATH - install LXD on the host first"
	if err == nil || err.Error() != want {
		t.Fatalf("Build error = %v, want %q", err, want)
	}
	assertEvents(t, runtime.events, []string{"available"})
}

func assertEvents(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("events:\n got: %q\nwant: %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d:\n got: %q\nwant: %q", i, got[i], want[i])
		}
	}
}
