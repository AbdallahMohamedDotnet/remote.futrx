package codex

import (
	"context"
	"log"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	agentruntime "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/runtime"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type ProjectResolver interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	Start(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	ListSecrets(ctx context.Context, id serviceproject.ID) ([]serviceproject.Secret, error)
}

type ContainerPreparer interface {
	EnsureCodex(ctx context.Context, containerName string) error
	EnsureCodexAuth(ctx context.Context, containerName string) error
	EnsureAgentInstructions(ctx context.Context, containerName string) error
	EnsureWorkspaceClaudeMirror(ctx context.Context, containerName string) error
	EnsureBootAutostart(ctx context.Context, containerName string) error
	SyncCodexAuthFromContainer(ctx context.Context, containerName string) error
}

type Provider struct {
	projects   ProjectResolver
	containers ContainerPreparer
}

func New(projects ProjectResolver, containers ContainerPreparer) *Provider {
	return &Provider{projects: projects, containers: containers}
}

func (p *Provider) ID() agent.ProviderID {
	return agent.ProviderCodex
}

func (p *Provider) Parser(req agent.RunRequest) agent.LineParser {
	return NewParser(req)
}

func (p *Provider) Run(ctx context.Context, req agent.RunRequest, emit func(agent.Event)) error {
	if emit == nil {
		emit = func(agent.Event) {}
	}
	if req.Provider == "" {
		req.Provider = agent.ProviderCodex
	}

	cmd, containerName, err := p.buildCmd(ctx, req, p.args(req), emit)
	if err != nil {
		return err
	}
	err = agentruntime.RunProcess(ctx, cmd, p.Parser(req), emit, agentruntime.ProcessOptions{
		Name:           "codex",
		LogID:          req.ConversationID,
		Provider:       agent.ProviderCodex,
		ConversationID: req.ConversationID,
	})
	if err == nil && containerName != "" && p.containers != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if syncErr := p.containers.SyncCodexAuthFromContainer(syncCtx, containerName); syncErr != nil {
			log.Printf("codex[%s] sync auth from %s: %v", req.ConversationID, containerName, syncErr)
		}
	}
	return err
}
