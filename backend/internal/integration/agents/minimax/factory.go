package minimax

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/service/agent/module"
)

// NewFactory returns MiniMax's project-scoped module. The provider uses the
// Codex app-server harness with a separate home and MiniMax Responses endpoint.
func NewFactory() (agentmodule.Factory, error) {
	profile := Profile()
	return agentmodule.NewFactory(agentmodule.Descriptor{
		ID:               agent.ProviderMiniMax,
		Label:            "MiniMax",
		ExecutionScopes:  []agentmodule.ExecutionScope{agentmodule.ScopeProject},
		Auth:             agentmodule.AuthExternal,
		AuthInstructions: "Add a MiniMax API key as `MINIMAX_API_KEY` in each project's Secrets settings.",
		Features: agentmodule.Features{
			Sessions:       agentmodule.SessionSupport{Resume: true, Fork: true},
			Skills:         agentmodule.SkillsDollarMention,
			BrowserTools:   true,
			ScheduledTools: true,
		},
	}, &profile, func(deps agentmodule.Dependencies, validatedProfile *provisioning.Profile) (agentmodule.Components, error) {
		binding := agentauth.NewExternalBinding(agent.ProviderMiniMax)
		return agentmodule.Components{
			Provider: newProvider(deps.ProjectPreparer, validatedProfile.CLI.Binary),
			Auth:     &binding,
		}, nil
	}, agentmodule.WithProjectPreparation(agentmodule.ProjectPreparationPolicy{
		SkillLinksRequired: true,
		BrowserAssets:      true,
		BrowserMCPRuntime:  true,
	}))
}

var (
	_ agent.Provider             = (*Provider)(nil)
	_ agentmodule.FactoryBuilder = NewFactory
)
