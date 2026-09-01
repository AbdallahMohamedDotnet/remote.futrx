package minimax

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

const miniMaxAPIKeyEnvironment = "MINIMAX_API_KEY"

var (
	ErrProjectRequired      = errors.New("MiniMax is available in project chats")
	ErrMiniMaxAPIKeyMissing = errors.New("MiniMax API key is not configured; add MINIMAX_API_KEY in this project's Secrets settings")
)

func (p *Provider) args(req agent.RunRequest) []string {
	args := []string{"app-server"}
	args = append(args, miniMaxConfigArgs()...)
	if req.EnableBrowser {
		args = append(args,
			"-c", `mcp_servers.browser.command="npx"`,
			"-c", `mcp_servers.browser.args=["@playwright/mcp","--cdp-endpoint","http://127.0.0.1:9222","--caps=vision"]`,
		)
	}
	return args
}

func miniMaxConfigArgs() []string {
	return []string{
		"-c", `model="MiniMax-M3"`,
		"-c", `model_provider="minimax"`,
		"-c", `model_context_window=1000000`,
		"-c", `model_catalog_json="/root/.minimax/model-catalog.json"`,
		"-c", `model_providers.minimax.name="MiniMax"`,
		"-c", `model_providers.minimax.base_url="https://api.minimax.io/v1"`,
		"-c", `model_providers.minimax.env_key="MINIMAX_API_KEY"`,
		"-c", `model_providers.minimax.wire_api="responses"`,
	}
}

func (p *Provider) buildCmd(
	ctx context.Context,
	req agent.RunRequest,
	args []string,
	emit func(agent.Event),
) (*exec.Cmd, error) {
	if req.ProjectID == "" || p.projectPreparer == nil {
		return nil, ErrProjectRequired
	}
	project, err := p.projectPreparer.Prepare(ctx, agent.ProjectPreparationRequest{
		ProjectID:           agent.ProjectID(req.ProjectID),
		ConversationID:      req.ConversationID,
		EnableBrowser:       req.EnableBrowser,
		EnableScheduleTools: req.EnableScheduleTools,
	}, emit)
	if err != nil {
		return nil, err
	}
	if !hasMiniMaxAPIKey(project.Secrets) {
		return nil, ErrMiniMaxAPIKeyMissing
	}
	return agentruntime.BuildContainerCommand(ctx, agentruntime.ContainerCommandSpec{
		ContainerName:      project.ContainerName,
		Secrets:            project.Secrets,
		ExcludedSecrets:    []string{"HOME", "CODEX_HOME", "OPENAI_API_KEY"},
		SuffixEnvironment:  []string{"HOME=/root", "CODEX_HOME=" + containerMiniMaxHome, "OPENAI_API_KEY="},
		RuntimeEnvironment: req.RuntimeEnv,
		Binary:             p.binary,
		Arguments:          args,
	}), nil
}

func hasMiniMaxAPIKey(secrets []agent.ProjectSecret) bool {
	for _, secret := range secrets {
		if secret.Key == miniMaxAPIKeyEnvironment && strings.TrimSpace(secret.Value) != "" {
			return true
		}
	}
	return false
}
