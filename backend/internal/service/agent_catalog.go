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

// agentDefinition is the single composition catalog for one agent. Adding an
// agent means adding one entry with its provider, provisioning profile, and
// shared-auth caller configuration.
type agentDefinition struct {
	profile     func() provisioning.Profile
	provider    func(*serviceproject.Service, provisioning.ContainerDependencies) agent.Provider
	authBinding func() agentauth.Binding
}

func agentDefinitions() []agentDefinition {
	return []agentDefinition{
		{
			profile: claudeagent.Profile,
			provider: func(projects *serviceproject.Service, containerDeps provisioning.ContainerDependencies) agent.Provider {
				return claudeagent.New(projects, containerDeps)
			},
			authBinding: func() agentauth.Binding {
				return agentauth.NewCodeBinding(agent.ProviderClaude, claudeagent.NewAuth())
			},
		},
		{
			profile: codexagent.Profile,
			provider: func(projects *serviceproject.Service, containerDeps provisioning.ContainerDependencies) agent.Provider {
				return codexagent.New(projects, containerDeps)
			},
			authBinding: func() agentauth.Binding {
				return agentauth.NewDeviceBinding(agent.ProviderCodex, codexagent.NewAuth())
			},
		},
		{
			profile: kimiagent.Profile,
			provider: func(projects *serviceproject.Service, containerDeps provisioning.ContainerDependencies) agent.Provider {
				return kimiagent.New(projects, containerDeps)
			},
			authBinding: func() agentauth.Binding {
				return agentauth.NewDeviceBinding(agent.ProviderKimi, kimiagent.NewAuth())
			},
		},
	}
}

// AgentProfiles returns the container-facing profiles from the same catalog
// used to register providers and their auth callers.
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
