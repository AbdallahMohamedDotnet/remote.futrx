package claude

import (
	"context"
	"log"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	agentruntime "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/runtime"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type ProjectResolver interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	Start(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	ListSecrets(ctx context.Context, id serviceproject.ID) ([]serviceproject.Secret, error)
}

type Provider struct {
	projects      ProjectResolver
	containerDeps provisioning.ContainerDependencies
	profile       provisioning.Profile
}

func New(projects ProjectResolver, containerDeps provisioning.ContainerDependencies) *Provider {
	return &Provider{
		projects:      projects,
		containerDeps: containerDeps,
		profile:       Profile(),
	}
}

func (p *Provider) ID() agent.ProviderID {
	return agent.ProviderClaude
}

func (p *Provider) Parser(req agent.RunRequest) agent.LineParser {
	return NewParser(req)
}

func (p *Provider) Run(ctx context.Context, req agent.RunRequest, emit func(agent.Event)) error {
	if emit == nil {
		emit = func(agent.Event) {}
	}
	if req.Provider == "" {
		req.Provider = agent.ProviderClaude
	}

	cmd, containerName, err := p.buildCmd(ctx, req, p.args(req), emit)
	if err != nil {
		return err
	}
	err = agentruntime.RunProcess(ctx, cmd, p.Parser(req), emit, agentruntime.ProcessOptions{
		Name:           "claude",
		LogID:          req.ConversationID,
		Provider:       agent.ProviderClaude,
		ConversationID: req.ConversationID,
	})
	if err == nil && containerName != "" && p.containerDeps.Credentials != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if syncErr := p.containerDeps.Credentials.SyncFromContainer(syncCtx, containerName, p.profile.Credentials); syncErr != nil {
			log.Printf("claude[%s] sync auth from %s: %v", req.ConversationID, containerName, syncErr)
		}
	}
	return err
}
