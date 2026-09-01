package minimax

import (
	"context"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codex"
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
	req.Model = miniMaxModel
	req.Preferences.ReasoningEffort = miniMaxReasoningEffort(req.Preferences.ReasoningEffort)
	req.Preferences.ServiceTier = ""

	cmd, err := p.buildCmd(ctx, req, p.args(req), emit)
	if err != nil {
		return err
	}
	return codex.RunAppServer(ctx, cmd, req, emit)
}
