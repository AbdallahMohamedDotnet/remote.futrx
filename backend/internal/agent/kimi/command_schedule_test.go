package kimi

import (
	"context"
	"slices"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

func TestBuildCmdPassesRuntimeEnvironmentOnHostAndIntoContainer(t *testing.T) {
	runtimeEnv := map[string]string{
		"REMOTE_SCHEDULE_API":   "https://remote.test/agent-api/schedules",
		"REMOTE_SCHEDULE_GRANT": "short-lived-grant",
	}

	hostProvider := New(nil, provisioning.ContainerDependencies{})
	hostRequest := agent.RunRequest{
		Prompt:     "resume",
		Cwd:        t.TempDir(),
		RuntimeEnv: runtimeEnv,
	}
	hostCmd, containerName, err := hostProvider.buildCmd(
		context.Background(),
		hostRequest,
		hostProvider.args(hostRequest),
		func(agent.Event) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if containerName != "" {
		t.Fatalf("host command container = %q", containerName)
	}
	for key, value := range runtimeEnv {
		if !slices.Contains(hostCmd.Env, key+"="+value) {
			t.Fatalf("host command env missing %s: %#v", key, hostCmd.Env)
		}
	}

	project := serviceproject.Meta{
		ID:            serviceproject.ID("abcd"),
		ContainerName: "schedule-project",
		Status:        serviceproject.StatusRunning,
	}
	containerProvider := New(
		fakeKimiScheduleProjects{
			project: project,
			secrets: []serviceproject.Secret{{
				Key:   "REMOTE_SCHEDULE_API",
				Value: "https://attacker.invalid",
			}},
		},
		provisioning.ContainerDependencies{},
	)
	containerRequest := agent.RunRequest{
		Prompt:     "resume",
		ProjectID:  string(project.ID),
		RuntimeEnv: runtimeEnv,
	}
	containerCmd, containerName, err := containerProvider.buildCmd(
		context.Background(),
		containerRequest,
		containerProvider.args(containerRequest),
		func(agent.Event) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if containerName != project.ContainerName {
		t.Fatalf("container name = %q, want %q", containerName, project.ContainerName)
	}
	for key, value := range runtimeEnv {
		requireKimiArgPair(t, containerCmd.Args, "--env", key+"="+value)
	}
	if slices.Contains(containerCmd.Args, "REMOTE_SCHEDULE_API=https://attacker.invalid") {
		t.Fatal("project secret overrode the backend-issued schedule API")
	}
}

type fakeKimiScheduleProjects struct {
	project serviceproject.Meta
	secrets []serviceproject.Secret
}

func (f fakeKimiScheduleProjects) Get(
	context.Context,
	serviceproject.ID,
) (serviceproject.Meta, error) {
	return f.project, nil
}

func (f fakeKimiScheduleProjects) Start(
	context.Context,
	serviceproject.ID,
) (serviceproject.Meta, error) {
	return f.project, nil
}

func (f fakeKimiScheduleProjects) ListSecrets(
	context.Context,
	serviceproject.ID,
) ([]serviceproject.Secret, error) {
	return f.secrets, nil
}

func requireKimiArgPair(t *testing.T, args []string, first, second string) {
	t.Helper()
	for index := 0; index+1 < len(args); index++ {
		if args[index] == first && args[index+1] == second {
			return
		}
	}
	t.Fatalf("command args missing pair %q %q: %#v", first, second, args)
}
