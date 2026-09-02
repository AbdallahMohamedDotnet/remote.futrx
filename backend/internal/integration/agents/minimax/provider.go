package minimax

import (
	"context"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codexharness"
)

type Provider struct {
	projectPreparer agent.ProjectPreparer
	binary          string
}

func newProvider(projectPreparer agent.ProjectPreparer, binary string) *Provider {
	return &Provider{projectPreparer: projectPreparer, binary: binary}
}

func (p *Provider) ID() agent.ProviderID {
	return agent.ProviderMiniMax
}

func (p *Provider) Run(ctx context.Context, req agent.RunRequest, emit func(agent.Event)) error {
	req.Provider = agent.ProviderMiniMax
	req.Model = configconstants.MiniMaxModel
	req.Preferences.ReasoningEffort = miniMaxReasoningEffort(req.Preferences.ReasoningEffort)
	req.Preferences.ServiceTier = ""

	cmd, err := p.buildCmd(ctx, req, p.args(req), emit)
	if err != nil {
		return err
	}
	return codexharness.Run(ctx, cmd, req, configconstants.MiniMaxLabel, emit)
}
