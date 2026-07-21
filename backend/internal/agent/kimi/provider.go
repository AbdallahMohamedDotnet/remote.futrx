package kimi

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
	projects   ProjectResolver
	containers provisioning.Container
	profile    provisioning.Profile
}

func New(projects ProjectResolver, containers provisioning.Container) *Provider {
	return &Provider{projects: projects, containers: containers, profile: Profile()}
}

func (p *Provider) ID() agent.ProviderID {
	return agent.ProviderKimi
}

func (p *Provider) Parser(req agent.RunRequest) agent.LineParser {
	return NewParser(req)
}

func (p *Provider) Run(ctx context.Context, req agent.RunRequest, emit func(agent.Event)) error {
	if emit == nil {
		emit = func(agent.Event) {}
	}
	if req.Provider == "" {
		req.Provider = agent.ProviderKimi
	}
	// kimi-code has no fork primitive; a forked chat simply starts fresh.
	if req.Fork {
		req.ResumeID = ""
	}

	cmd, containerName, err := p.buildCmd(ctx, req, p.args(req), emit)
	if err != nil {
		return err
	}
	err = agentruntime.RunProcess(ctx, cmd, p.Parser(req), emit, agentruntime.ProcessOptions{
		Name:           "kimi",
		LogID:          req.ConversationID,
		Provider:       agent.ProviderKimi,
		ConversationID: req.ConversationID,
	})
	if err == nil && containerName != "" && p.containers != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if syncErr := p.containers.SyncCredentialsFromContainer(syncCtx, containerName, p.profile.Credentials); syncErr != nil {
			log.Printf("kimi[%s] sync auth from %s: %v", req.ConversationID, containerName, syncErr)
		}
	}
	return err
}
