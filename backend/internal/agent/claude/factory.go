package claude

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/agent/auth"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/agent/module"
)

// Factory returns Claude's complete module definition. The profile and
// descriptor are immutable policy; runtime and auth state are created afresh
// each time the catalog builds an application runtime.
func Factory() (agentmodule.Factory, error) {
	profile := Profile()
	return agentmodule.NewFactory(agentmodule.Descriptor{
		ID:                  agent.ProviderClaude,
		Label:               "Claude",
		ExecutionScopes:     []agentmodule.ExecutionScope{agentmodule.ScopeHost, agentmodule.ScopeProject},
		Auth:                agentmodule.AuthManagedCode,
		AuthInstructions:    "Starts `claude auth login --claudeai` on the host. Sign in with your Anthropic subscription; credentials are shared with project containers.",
		SatisfiesAccessGate: true,
		LegacySkillRoots: []string{
			"/root/.claude/skills",
		},
		Features: agentmodule.Features{
			Sessions:       agentmodule.SessionSupport{Resume: true, Fork: true},
			Skills:         agentmodule.SkillsSlashCommand,
			BrowserTools:   true,
			ScheduledTools: true,
		},
		Profile: &profile,
	}, func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
		binding := agentauth.NewCodeBinding(agent.ProviderClaude, NewAuth())
		return agentmodule.Components{
			Provider: New(deps.Projects, deps.Containers),
			Auth:     &binding,
		}, nil
	})
}

var _ agent.Provider = (*Provider)(nil)
