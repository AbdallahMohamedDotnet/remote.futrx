package service

import (
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	claudeagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/claude"
	codexagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/codex"
	kimiagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/kimi"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	agentauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/agent/auth"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// AgentAuthCallers are the agent-facing adapters configured over the shared
// service/agent/auth lifecycle implementation.
type AgentAuthCallers struct {
	Claude *claudeagent.Auth
	Codex  *codexagent.Auth
	Kimi   *kimiagent.Auth

	bindings []agentauth.Binding
}

func (c AgentAuthCallers) Bindings() []agentauth.Binding {
	return append([]agentauth.Binding(nil), c.bindings...)
}

// agentDefinition is the single composition catalog for one agent. Adding an
// agent means adding one entry with its provider, provisioning profile, and
// shared-auth caller configuration.
type agentDefinition struct {
	profile       func() provisioning.Profile
	provider      func(*serviceproject.Service, provisioning.Container) agent.Provider
	configureAuth func(*AgentAuthCallers)
}

func agentDefinitions() []agentDefinition {
	return []agentDefinition{
		{
			profile: claudeagent.Profile,
			provider: func(projects *serviceproject.Service, containers provisioning.Container) agent.Provider {
				return claudeagent.New(projects, containers)
			},
			configureAuth: func(auth *AgentAuthCallers) {
				auth.Claude = claudeagent.NewAuth()
				auth.bindings = append(auth.bindings,
					agentauth.NewCodeBinding(agent.ProviderClaude, auth.Claude))
			},
		},
		{
			profile: codexagent.Profile,
			provider: func(projects *serviceproject.Service, containers provisioning.Container) agent.Provider {
				return codexagent.New(projects, containers)
			},
			configureAuth: func(auth *AgentAuthCallers) {
				auth.Codex = codexagent.NewAuth()
				auth.bindings = append(auth.bindings,
					agentauth.NewDeviceBinding(agent.ProviderCodex, auth.Codex))
			},
		},
		{
			profile: kimiagent.Profile,
			provider: func(projects *serviceproject.Service, containers provisioning.Container) agent.Provider {
				return kimiagent.New(projects, containers)
			},
			configureAuth: func(auth *AgentAuthCallers) {
				auth.Kimi = kimiagent.NewAuth()
				auth.bindings = append(auth.bindings,
					agentauth.NewDeviceBinding(agent.ProviderKimi, auth.Kimi))
			},
		},
	}
}

// AgentProfiles returns the container-facing profiles from the same catalog
// used to register providers and configure their auth callers.
func AgentProfiles() []provisioning.Profile {
	return profilesFromDefinitions(agentDefinitions())
}

func profilesFromDefinitions(definitions []agentDefinition) []provisioning.Profile {
	profiles := make([]provisioning.Profile, 0, len(definitions))
	for _, definition := range definitions {
		profiles = append(profiles, definition.profile())
	}
	return profiles
}
