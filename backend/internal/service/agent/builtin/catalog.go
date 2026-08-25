// Package builtin is the explicit composition root for Remote's compiled-in
// agent integrations. Registering an implemented integration requires one
// reviewed entry here; construction stays deterministic and free of init hooks.
package builtin

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	antigravityagent "github.com/futrx-com/remote.futrx.com/internal/agent/antigravity"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/agent/auth"
	claudeagent "github.com/futrx-com/remote.futrx.com/internal/agent/claude"
	codexagent "github.com/futrx-com/remote.futrx.com/internal/agent/codex"
	kimiagent "github.com/futrx-com/remote.futrx.com/internal/agent/kimi"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/agent/module"
)

func Catalog() (*agentmodule.Catalog, error) {
	claudeProfile := claudeagent.Profile()
	codexProfile := codexagent.Profile()
	kimiProfile := kimiagent.Profile()
	antigravityProfile := antigravityagent.Profile()

	specs := []struct {
		descriptor agentmodule.Descriptor
		build      agentmodule.BuildFunc
	}{
		{
			descriptor: agentmodule.Descriptor{
				ID:                  agent.ProviderClaude,
				Label:               "Claude",
				ExecutionScopes:     allScopes(),
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
				Profile: &claudeProfile,
			},
			build: func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
				binding := agentauth.NewCodeBinding(agent.ProviderClaude, claudeagent.NewAuth())
				return agentmodule.Components{
					Provider: claudeagent.New(deps.Projects, deps.Containers),
					Auth:     &binding,
				}, nil
			},
		},
		{
			descriptor: agentmodule.Descriptor{
				ID:                  agent.ProviderCodex,
				Label:               "Codex",
				Default:             true,
				ExecutionScopes:     allScopes(),
				Auth:                agentmodule.AuthManagedDevice,
				AuthInstructions:    "Starts `codex login --device-auth` on the host. Sign in with ChatGPT so Codex uses subscription limits instead of API-key billing.",
				SatisfiesAccessGate: true,
				LegacySkillRoots: []string{
					"/root/.codex/skills",
				},
				Features: agentmodule.Features{
					Sessions:       agentmodule.SessionSupport{Resume: true, Fork: true},
					Skills:         agentmodule.SkillsDollarMention,
					BrowserTools:   true,
					ScheduledTools: true,
				},
				Profile: &codexProfile,
			},
			build: func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
				auth := codexagent.NewAuth()
				binding := agentauth.NewDeviceBinding(agent.ProviderCodex, auth).WithWarning(func() string {
					if auth.Status().UsesAPIKey {
						return "Codex is logged in with an API key. Sign in with ChatGPT to use subscription limits."
					}
					return ""
				})
				return agentmodule.Components{
					Provider: codexagent.New(deps.Projects, deps.Containers),
					Auth:     &binding,
				}, nil
			},
		},
		{
			descriptor: agentmodule.Descriptor{
				ID:                  agent.ProviderKimi,
				Label:               "Kimi",
				ExecutionScopes:     allScopes(),
				Auth:                agentmodule.AuthManagedDevice,
				AuthInstructions:    "Starts `kimi login` on the host. Sign in with your Kimi Code subscription using the displayed device code.",
				SatisfiesAccessGate: true,
				Features: agentmodule.Features{
					Sessions:       agentmodule.SessionSupport{Resume: true},
					Skills:         agentmodule.SkillsInstructions,
					ScheduledTools: true,
				},
				Profile: &kimiProfile,
			},
			build: func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
				binding := agentauth.NewDeviceBinding(agent.ProviderKimi, kimiagent.NewAuth())
				return agentmodule.Components{
					Provider: kimiagent.New(deps.Projects, deps.Containers),
					Auth:     &binding,
				}, nil
			},
		},
		{
			descriptor: agentmodule.Descriptor{
				ID:               agent.ProviderAntigravity,
				Label:            "Antigravity",
				ExecutionScopes:  allScopes(),
				Auth:             agentmodule.AuthExternal,
				AuthInstructions: "Open the project terminal, run `agy`, and complete its sign-in flow.",
				Features: agentmodule.Features{
					Sessions:       agentmodule.SessionSupport{Resume: true},
					Skills:         agentmodule.SkillsInstructions,
					ScheduledTools: true,
				},
				Profile: &antigravityProfile,
			},
			build: func(deps agentmodule.Dependencies) (agentmodule.Components, error) {
				binding := agentauth.NewExternalBinding(agent.ProviderAntigravity)
				return agentmodule.Components{
					Provider: antigravityagent.New(deps.Projects, deps.Containers),
					Auth:     &binding,
				}, nil
			},
		},
	}

	factories := make([]agentmodule.Factory, 0, len(specs))
	for _, spec := range specs {
		factory, err := agentmodule.NewFactory(spec.descriptor, spec.build)
		if err != nil {
			return nil, err
		}
		factories = append(factories, factory)
	}
	return agentmodule.NewCatalog(factories...)
}

func allScopes() []agentmodule.ExecutionScope {
	return []agentmodule.ExecutionScope{agentmodule.ScopeHost, agentmodule.ScopeProject}
}

// Compile-time checks keep the built-in constructors aligned with the module
// runtime contract before catalog validation runs at application startup.
var (
	_ agent.Provider = (*claudeagent.Provider)(nil)
	_ agent.Provider = (*codexagent.Provider)(nil)
	_ agent.Provider = (*kimiagent.Provider)(nil)
	_ agent.Provider = (*antigravityagent.Provider)(nil)
)
