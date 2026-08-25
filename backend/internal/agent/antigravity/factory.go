package antigravity

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/agent/auth"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/agent/module"
)

// Factory returns Antigravity's complete module definition. Authentication is
// provider-owned and external, while the shared profile supplies deterministic
// host and project provisioning policy.
func Factory() (agentmodule.Factory, error) {
	profile := Profile()
	return agentmodule.NewFactory(agentmodule.Descriptor{
		ID:               agent.ProviderAntigravity,
		Label:            "Antigravity",
		ExecutionScopes:  []agentmodule.ExecutionScope{agentmodule.ScopeHost, agentmodule.ScopeProject},
		Auth:             agentmodule.AuthExternal,
		AuthInstructions: "Open the project terminal, run `agy`, and complete its sign-in flow.",
		Features: agentmodule.Features{
			Sessions:       agentmodule.SessionSupport{Resume: true},
			Skills:         agentmodule.SkillsInstructions,
			ScheduledTools: true,
		},
		Profile: &profile,
	}, func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
		binding := agentauth.NewExternalBinding(agent.ProviderAntigravity)
		return agentmodule.Components{
			Provider: New(deps.Projects, deps.Containers),
			Auth:     &binding,
		}, nil
	})
}

var _ agent.Provider = (*Provider)(nil)
