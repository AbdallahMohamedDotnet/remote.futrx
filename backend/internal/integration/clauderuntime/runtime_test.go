package clauderuntime

import (
	"context"
	"io"
	"os/exec"
	"reflect"
	"testing"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
)

func TestRuntimeBuildsHostClaudeCommand(t *testing.T) {
	runner := &recordingRunner{}
	runtime := NewWithRunner(nil, nil, runner)

	err := runtime.Run(
		context.Background(),
		prompt.ClaudeRunRequest{
			ChatID:           "abcd",
			Args:             []string{"-p", "--model", "sonnet"},
			Prompt:           "hello",
			HostCwd:          "/tmp/work",
			CurrentSessionID: "session-1",
		},
		func(prompt.ChatEvent) {},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if runner.chatID != "abcd" {
		t.Fatalf("chat id = %q", runner.chatID)
	}
	if runner.currentSessionID != "session-1" {
		t.Fatalf("session id = %q", runner.currentSessionID)
	}
	if got, want := runner.cmd.Args, []string{"claude", "-p", "--model", "sonnet"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if runner.cmd.Dir != "/tmp/work" {
		t.Fatalf("dir = %q", runner.cmd.Dir)
	}
	if !containsEnv(runner.cmd.Env, "IS_SANDBOX=1") {
		t.Fatalf("IS_SANDBOX env missing: %#v", runner.cmd.Env)
	}
	if got := readStdin(t, runner.cmd); got != "hello" {
		t.Fatalf("stdin = %q", got)
	}
}

func TestRuntimePreparesProjectAndBuildsLXCCommand(t *testing.T) {
	runner := &recordingRunner{}
	projects := &fakeProjects{
		meta: serviceproject.Meta{
			ID:            "beef",
			ContainerName: "proj-demo",
			Status:        serviceproject.StatusStopped,
		},
	}
	containers := &fakeContainers{}
	runtime := NewWithRunner(projects, containers, runner)

	var subtypes []string
	err := runtime.Run(
		context.Background(),
		prompt.ClaudeRunRequest{
			ChatID:    "abcd",
			ProjectID: "beef",
			Args:      []string{"-p"},
			Prompt:    "inside",
		},
		func(ev prompt.ChatEvent) {
			if ev.Type == "system" {
				subtypes = append(subtypes, ev.Subtype)
			}
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := projects.started, []serviceproject.ID{"beef"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("started = %#v, want %#v", got, want)
	}
	if got, want := containers.calls, []string{
		"EnsureClaude:proj-demo",
		"EnsureClaudeAuth:proj-demo",
		"EnsureClaudeMD:proj-demo",
		"EnsureBootAutostart:proj-demo",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("container calls = %#v, want %#v", got, want)
	}
	if got, want := subtypes, []string{"container_starting", "container_preparing"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("subtypes = %#v, want %#v", got, want)
	}
	if got, want := runner.cmd.Args, []string{
		"lxc", "exec",
		"--cwd", "/workspace",
		"--env", "IS_SANDBOX=1",
		"--env", "HOME=/root",
		"proj-demo",
		"--",
		"claude",
		"-p",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got := readStdin(t, runner.cmd); got != "inside" {
		t.Fatalf("stdin = %q", got)
	}
}

type recordingRunner struct {
	chatID           servicechat.ID
	cmd              *exec.Cmd
	currentSessionID string
}

func (r *recordingRunner) Run(
	ctx context.Context,
	id servicechat.ID,
	cmd *exec.Cmd,
	currentSessionID string,
	emit func(prompt.ChatEvent),
	updateSession func(sessionID, model string),
) error {
	r.chatID = id
	r.cmd = cmd
	r.currentSessionID = currentSessionID
	return nil
}

type fakeProjects struct {
	meta    serviceproject.Meta
	started []serviceproject.ID
}

func (p *fakeProjects) Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error) {
	return p.meta, nil
}

func (p *fakeProjects) Start(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error) {
	p.started = append(p.started, id)
	p.meta.Status = serviceproject.StatusRunning
	return p.meta, nil
}

type fakeContainers struct {
	calls []string
}

func (c *fakeContainers) EnsureClaude(ctx context.Context, containerName string) error {
	c.calls = append(c.calls, "EnsureClaude:"+containerName)
	return nil
}

func (c *fakeContainers) EnsureClaudeAuth(ctx context.Context, containerName string) error {
	c.calls = append(c.calls, "EnsureClaudeAuth:"+containerName)
	return nil
}

func (c *fakeContainers) EnsureClaudeMD(ctx context.Context, containerName string) error {
	c.calls = append(c.calls, "EnsureClaudeMD:"+containerName)
	return nil
}

func (c *fakeContainers) EnsureBootAutostart(ctx context.Context, containerName string) error {
	c.calls = append(c.calls, "EnsureBootAutostart:"+containerName)
	return nil
}

func readStdin(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	data, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsEnv(env []string, want string) bool {
	for _, v := range env {
		if v == want {
			return true
		}
	}
	return false
}
