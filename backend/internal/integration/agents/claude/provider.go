package claude

import (
	"context"
	"log"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

type Provider struct {
	projectPreparer       agent.ProjectPreparer
	credentialCollector   provisioning.CredentialCollector
	profile               provisioning.Profile
	credentialSyncTimeout time.Duration
	accounts              *Auth
}

func newProvider(
	projectPreparer agent.ProjectPreparer,
	credentialCollector provisioning.CredentialCollector,
	profile provisioning.Profile,
	credentialSyncTimeout time.Duration,
	accounts *Auth,
) *Provider {
	return &Provider{
		projectPreparer:       projectPreparer,
		credentialCollector:   credentialCollector,
		profile:               profile.Clone(),
		credentialSyncTimeout: credentialSyncTimeout,
		accounts:              accounts,
	}
}

func (p *Provider) ID() agent.ProviderID {
	return agent.ProviderClaude
}

func (p *Provider) Parser(req agent.RunRequest) agent.LineParser {
	return NewParser(req)
}

func (p *Provider) Run(ctx context.Context, req agent.RunRequest, emit func(agent.Event)) error {
	var releaseAccount func()
	if p.accounts != nil {
		var err error
		releaseAccount, err = p.accounts.BeginRun()
		if err != nil {
			return err
		}
		defer releaseAccount()
	}
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
	credentialReady := err == nil && containerName == ""
	if err == nil && containerName != "" && p.credentialCollector != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), p.credentialSyncTimeout)
		defer cancel()
		if syncErr := p.credentialCollector.SyncFromContainer(syncCtx, containerName, p.profile.Credentials); syncErr != nil {
			log.Printf("claude[%s] sync auth from %s: %v", req.ConversationID, containerName, syncErr)
		} else {
			credentialReady = true
		}
	}
	if credentialReady && p.accounts != nil {
		captureCtx, cancel := context.WithTimeout(context.Background(), p.credentialSyncTimeout)
		defer cancel()
		if captureErr := p.accounts.CaptureActiveCredential(captureCtx); captureErr != nil {
			log.Printf("claude[%s] retain refreshed account: %v", req.ConversationID, captureErr)
		}
	}
	return err
}
