package kimi

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/agent/module"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

// Factory returns Kimi's complete module definition, including its runtime,
// authentication flow, feature declarations, and provisioning profile.
func Factory() (agentmodule.Factory, error) {
	profile := Profile()
	return agentmodule.NewFactory(agentmodule.Descriptor{
		ID:                  agent.ProviderKimi,
		Label:               "Kimi",
		ExecutionScopes:     []agentmodule.ExecutionScope{agentmodule.ScopeHost, agentmodule.ScopeProject},
		Auth:                agentmodule.AuthManagedDevice,
		AuthInstructions:    "Starts `kimi login` on the host. Sign in with your Kimi Code subscription using the displayed device code.",
		SatisfiesAccessGate: true,
		Features: agentmodule.Features{
			Sessions:       agentmodule.SessionSupport{Resume: true},
			Skills:         agentmodule.SkillsInstructions,
			ScheduledTools: true,
		},
	}, &profile, func(deps agentmodule.Dependencies, validatedProfile *provisioning.Profile) (agentmodule.Components, error) {
		binding := agentauth.NewDeviceBinding(agent.ProviderKimi, NewAuth())
		return agentmodule.Components{
			Provider: newProvider(deps.Projects, deps.Containers, *validatedProfile, deps.CredentialSyncTimeout),
			Auth:     &binding,
		}, nil
	})
}

var (
	_ agent.Provider             = (*Provider)(nil)
	_ agentmodule.FactoryBuilder = Factory
)
