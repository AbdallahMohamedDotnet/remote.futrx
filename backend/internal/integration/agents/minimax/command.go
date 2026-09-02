package minimax

import (
	"context"
	"errors"
	"os/exec"
	"strconv"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codexharness"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

var (
	ErrProjectRequired      = errors.New("MiniMax is available in project chats")
	ErrMiniMaxAPIKeyMissing = errors.New("MiniMax API key is not configured; add it in Settings → Agent authentication")
)

func (p *Provider) args(req agent.RunRequest) []string {
	return codexharness.AppServerArgs(miniMaxConfigArgs(), req.EnableBrowser)
}

func miniMaxConfigArgs() []string {
	providerID := string(agent.ProviderMiniMax)
	return []string{
		"-c", `model="` + configconstants.MiniMaxModel + `"`,
		"-c", `model_provider="` + providerID + `"`,
		"-c", `model_context_window=` + strconv.Itoa(configconstants.MiniMaxModelContextWindow),
		"-c", `model_catalog_json="` + configconstants.MiniMaxContainerCatalog + `"`,
		"-c", `model_providers.` + providerID + `.name="` + configconstants.MiniMaxLabel + `"`,
		"-c", `model_providers.` + providerID + `.base_url="` + configconstants.MiniMaxAPIBaseURL + `"`,
		"-c", `model_providers.` + providerID + `.env_key="` + configconstants.MiniMaxAPIKeyEnvironment + `"`,
		"-c", `model_providers.` + providerID + `.wire_api="` + configconstants.MiniMaxWireAPI + `"`,
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
	if p.apiKeys == nil {
		return nil, ErrMiniMaxAPIKeyMissing
	}
	apiKey, ok := p.apiKeys.APIKey()
	if !ok {
		return nil, ErrMiniMaxAPIKeyMissing
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
	runtimeEnvironment := make(map[string]string, len(req.RuntimeEnv)+1)
	for key, value := range req.RuntimeEnv {
		runtimeEnvironment[key] = value
	}
	runtimeEnvironment[configconstants.MiniMaxAPIKeyEnvironment] = apiKey
	return agentruntime.BuildContainerCommand(ctx, agentruntime.ContainerCommandSpec{
		ContainerName: project.ContainerName,
		Secrets:       project.Secrets,
		ExcludedSecrets: []string{
			"HOME", "CODEX_HOME", "OPENAI_API_KEY", configconstants.MiniMaxAPIKeyEnvironment,
		},
		SuffixEnvironment:  []string{"HOME=/root", "CODEX_HOME=" + configconstants.MiniMaxContainerHome, "OPENAI_API_KEY="},
		RuntimeEnvironment: runtimeEnvironment,
		Binary:             p.binary,
		Arguments:          args,
	}), nil
}
