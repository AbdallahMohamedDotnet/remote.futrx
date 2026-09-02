package minimax

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codexharness"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

var (
	ErrProjectRequired      = errors.New("MiniMax is available in project chats")
	ErrMiniMaxAPIKeyMissing = errors.New("MiniMax API key is not configured; add MINIMAX_API_KEY in this project's Secrets settings")
)

func (p *Provider) args(req agent.RunRequest) []string {
	return codexharness.AppServerArgs(miniMaxConfigArgs(), req.EnableBrowser)
}

func miniMaxConfigArgs() []string {
	providerID := string(agent.ProviderMiniMax)
	return []string{
		"-c", `model="` + miniMaxModel + `"`,
		"-c", `model_provider="` + providerID + `"`,
		"-c", `model_context_window=` + strconv.Itoa(miniMaxModelContextWindow),
		"-c", `model_catalog_json="` + containerMiniMaxCatalog + `"`,
		"-c", `model_providers.` + providerID + `.name="` + miniMaxLabel + `"`,
		"-c", `model_providers.` + providerID + `.base_url="` + miniMaxAPIBaseURL + `"`,
		"-c", `model_providers.` + providerID + `.env_key="` + miniMaxAPIKeyEnvironment + `"`,
		"-c", `model_providers.` + providerID + `.wire_api="` + miniMaxWireAPI + `"`,
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
