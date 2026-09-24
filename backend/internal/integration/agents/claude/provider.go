package claude

import (
	"context"
	"log"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

type Provider struct {
	projectPreparer       agent.ProjectPreparer
	credentialCollector   provisioning.CredentialCollector
	profile               provisioning.Profile
	credentialSyncTimeout time.Duration
	// accounts is nil when saved accounts are unavailable.
	accounts *agentauth.AccountService
}

func newProvider(
	projectPreparer agent.ProjectPreparer,
	credentialCollector provisioning.CredentialCollector,
	profile provisioning.Profile,
	credentialSyncTimeout time.Duration,
	accounts *agentauth.AccountService,
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
	if p.accounts != nil {
		releaseAccount, err := p.accounts.BeginRun()
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
	// A successful run may leave a refreshed login on the host, written by
	// the host CLI or copied back from the project container.
	hostLoginTouched := err == nil && containerName == ""
	if err == nil && containerName != "" && p.credentialCollector != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), p.credentialSyncTimeout)
		defer cancel()
		if syncErr := p.credentialCollector.SyncFromContainer(syncCtx, containerName, p.profile.Credentials); syncErr != nil {
			log.Printf("claude[%s] sync auth from %s: %v", req.ConversationID, containerName, syncErr)
		}
		// Check the host login even when the copy failed: one of the two
		// files may already have been replaced.
		hostLoginTouched = true
	}
	// A login copied back from a project container is untrusted: the account
	// service saves it only once it validates as the active account.
	if hostLoginTouched && p.accounts != nil {
		captureCtx, cancel := context.WithTimeout(context.Background(), p.credentialSyncTimeout)
		defer cancel()
		if captureErr := p.accounts.CaptureAfterRun(captureCtx); captureErr != nil {
			log.Printf("claude[%s] keep account login after run: %v", req.ConversationID, captureErr)
		}
	}
	return err
}
