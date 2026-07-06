package codex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func TestArgsUseCodexExecJSONMode(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{Model: "gpt-5.5 [fast]"})

	want := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5.5",
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestCodexEnvStripsOpenAIAPIKey(t *testing.T) {
	env := codexEnv([]string{
		"HOME=/root",
		"OPENAI_API_KEY=sk-test",
	})

	for _, item := range env {
		if strings.HasPrefix(item, "OPENAI_API_KEY=") {
			t.Fatalf("OPENAI_API_KEY leaked into codex env: %#v", env)
		}
	}
	if !slices.Contains(env, "CODEX_HOME=/root/.codex") {
		t.Fatalf("CODEX_HOME missing from env: %#v", env)
	}
}

func TestArgsIncludeReasoningEffort(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{
		Config: map[string]any{"reasoningEffort": "high"},
	})

	want := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-c", "model_reasoning_effort=high",
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestArgsIgnoreInvalidReasoningEffort(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{
		Config: map[string]any{"reasoningEffort": "extreme"},
	})

	if slices.Contains(args, "-c") {
		t.Fatalf("invalid reasoning effort should not add config args: %#v", args)
	}
}

func TestArgsIncludeBrowserMCPConfig(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{EnableBrowser: true})

	want := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-c", `mcp_servers.browser.command="npx"`,
		"-c", `mcp_servers.browser.args=["@playwright/mcp","--cdp-endpoint","http://127.0.0.1:9222","--caps=vision"]`,
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestEnsureHostSubscriptionAuthRejectsAPIKeyAuth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"),
		[]byte(`{"auth_mode":"apikey","OPENAI_API_KEY":"sk-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ensureHostSubscriptionAuth(); err == nil {
		t.Fatal("expected API key auth to be rejected")
	}
}

func TestArgsResumeThread(t *testing.T) {
	provider := New(nil, nil)
	args := provider.args(agent.RunRequest{ResumeID: "thread-123"})

	want := []string{
		"exec",
		"resume",
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"thread-123",
		"-",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildCmdProvisionsBrowserMCPOnlyWhenEnabled(t *testing.T) {
	project := serviceproject.Meta{
		ID:            serviceproject.ID("abcd"),
		Slug:          "browser-project",
		ContainerName: "browser-project",
		Status:        serviceproject.StatusRunning,
	}
	projects := fakeCodexProjects{project: project}

	withoutBrowser := &fakeCodexContainers{}
	provider := New(projects, withoutBrowser)
	req := agent.RunRequest{ProjectID: string(project.ID)}
	if _, _, err := provider.buildCmd(context.Background(), req, provider.args(req), func(agent.Event) {}); err != nil {
		t.Fatal(err)
	}
	if withoutBrowser.agentBrowserMCPCalls != 0 {
		t.Fatalf("browser MCP provisioned without browser skill: %d", withoutBrowser.agentBrowserMCPCalls)
	}
	if withoutBrowser.agentBrowserCoreCalls != 0 {
		t.Fatalf("browser core started without browser skill: %d", withoutBrowser.agentBrowserCoreCalls)
	}

	withBrowser := &fakeCodexContainers{}
	provider = New(projects, withBrowser)
	req.EnableBrowser = true
	if _, _, err := provider.buildCmd(context.Background(), req, provider.args(req), func(agent.Event) {}); err != nil {
		t.Fatal(err)
	}
	if withBrowser.agentBrowserMCPCalls != 1 {
		t.Fatalf("browser MCP calls = %d, want 1", withBrowser.agentBrowserMCPCalls)
	}
	if withBrowser.agentBrowserCoreCalls != 1 {
		t.Fatalf("browser core calls = %d, want 1", withBrowser.agentBrowserCoreCalls)
	}
}

type fakeCodexProjects struct {
	project serviceproject.Meta
}

func (f fakeCodexProjects) Get(context.Context, serviceproject.ID) (serviceproject.Meta, error) {
	return f.project, nil
}

func (f fakeCodexProjects) Start(context.Context, serviceproject.ID) (serviceproject.Meta, error) {
	return f.project, nil
}

func (f fakeCodexProjects) ListSecrets(context.Context, serviceproject.ID) ([]serviceproject.Secret, error) {
	return nil, nil
}

type fakeCodexContainers struct {
	agentBrowserMCPCalls  int
	agentBrowserCoreCalls int
}

func (f *fakeCodexContainers) EnsureCodex(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureCodexAuth(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureAgentInstructions(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureWorkspaceSkillLinks(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureBrowserSkill(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureBrowserScript(context.Context, string) error { return nil }

func (f *fakeCodexContainers) EnsureAgentBrowserMCP(context.Context, string) error {
	f.agentBrowserMCPCalls++
	return nil
}

func (f *fakeCodexContainers) EnsureAgentBrowserCore(context.Context, string) error {
	f.agentBrowserCoreCalls++
	return nil
}

func (f *fakeCodexContainers) EnsureBootAutostart(context.Context, string) error { return nil }

func (f *fakeCodexContainers) SyncCodexAuthFromContainer(context.Context, string) error { return nil }
