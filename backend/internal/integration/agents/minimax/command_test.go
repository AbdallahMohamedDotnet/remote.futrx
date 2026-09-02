package minimax

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
)

func TestMiniMaxConfigUsesResponsesAndEnvironmentKey(t *testing.T) {
	args := miniMaxConfigArgs()
	for _, want := range []string{
		`model="MiniMax-M3"`,
		`model_provider="minimax"`,
		`model_context_window=1000000`,
		`model_catalog_json="/root/.minimax/model-catalog.json"`,
		`model_providers.minimax.base_url="https://api.minimax.io/v1"`,
		`model_providers.minimax.env_key="MINIMAX_API_KEY"`,
		`model_providers.minimax.wire_api="responses"`,
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("config args are missing %q: %#v", want, args)
		}
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "experimental_bearer_token") || strings.Contains(joined, "sk-") {
		t.Fatalf("config contains an embedded credential: %s", joined)
	}
}

func TestBuildCmdRequiresConfiguredMiniMaxAPIKey(t *testing.T) {
	provider := newProvider(miniMaxTestPreparer{
		project: agent.PreparedProject{ContainerName: "project-container"},
	}, miniMaxTestAPIKeys{}, "codex")
	_, err := provider.buildCmd(context.Background(), agent.RunRequest{ProjectID: "project"}, provider.args(agent.RunRequest{}), nil)
	if !errors.Is(err, ErrMiniMaxAPIKeyMissing) {
		t.Fatalf("error = %v, want ErrMiniMaxAPIKeyMissing", err)
	}
}

func TestBuildCmdUsesIsolatedHomeAndPreservesMiniMaxSecret(t *testing.T) {
	provider := newProvider(miniMaxTestPreparer{
		project: agent.PreparedProject{
			ContainerName: "project-container",
			Secrets: []agent.ProjectSecret{
				{Key: configconstants.MiniMaxAPIKeyEnvironment, Value: "project-key-must-not-pass"},
				{Key: "OPENAI_API_KEY", Value: "must-not-pass"},
				{Key: "HOME", Value: "/workspace/attacker-home"},
				{Key: "CODEX_HOME", Value: "/workspace/attacker-codex-home"},
			},
		},
	}, miniMaxTestAPIKeys{key: "managed-key"}, "codex")
	cmd, err := provider.buildCmd(
		context.Background(),
		agent.RunRequest{
			ProjectID: "project",
			RuntimeEnv: map[string]string{
				configconstants.MiniMaxAPIKeyEnvironment: "request-key-must-not-pass",
			},
		},
		provider.args(agent.RunRequest{}),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args, "\n")
	for _, want := range []string{
		"CODEX_HOME=/root/.minimax",
		"MINIMAX_API_KEY=managed-key",
		"OPENAI_API_KEY=",
		"model_providers.minimax.env_key=\"MINIMAX_API_KEY\"",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command is missing %q: %#v", want, cmd.Args)
		}
	}
	if strings.Contains(joined, "OPENAI_API_KEY=must-not-pass") {
		t.Fatalf("OpenAI key leaked into MiniMax command: %#v", cmd.Args)
	}
	if strings.Contains(joined, "MINIMAX_API_KEY=project-key-must-not-pass") {
		t.Fatalf("project key overrode the managed MiniMax key: %#v", cmd.Args)
	}
	if strings.Contains(joined, "MINIMAX_API_KEY=request-key-must-not-pass") {
		t.Fatalf("request environment overrode the managed MiniMax key: %#v", cmd.Args)
	}
	if strings.Contains(joined, "/workspace/attacker-home") ||
		strings.Contains(joined, "/workspace/attacker-codex-home") {
		t.Fatalf("project secrets overrode MiniMax's isolated home: %#v", cmd.Args)
	}
}

type miniMaxTestAPIKeys struct {
	key string
}

func (s miniMaxTestAPIKeys) APIKey() (string, bool) {
	return s.key, s.key != ""
}

func TestArgsWireBrowserThroughCodexHarness(t *testing.T) {
	args := (&Provider{}).args(agent.RunRequest{EnableBrowser: true})
	if !slices.Contains(args, `mcp_servers.browser.command="npx"`) {
		t.Fatalf("browser config = %#v", args)
	}
}

type miniMaxTestPreparer struct {
	project agent.PreparedProject
	err     error
}

func (p miniMaxTestPreparer) Prepare(
	context.Context,
	agent.ProjectPreparationRequest,
	func(agent.Event),
) (agent.PreparedProject, error) {
	return p.project, p.err
}
