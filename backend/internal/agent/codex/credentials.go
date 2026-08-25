package codex

import (
	"context"
	"errors"
)

var ErrCodexAPIKeyAuth = errors.New("Codex is logged in with an API key; run codex login with ChatGPT to use subscription limits")

func (p *Provider) ensureCredentials(ctx context.Context, containerName string) error {
	if codexAuthUsesAPIKey(p.profile.Credentials.Files[0].HostPath) {
		return ErrCodexAPIKeyAuth
	}
	return p.containerDeps.Credentials.Ensure(ctx, containerName, p.profile.Credentials)
}

func (p *Provider) syncCredentialsFromContainer(ctx context.Context, containerName string) error {
	if err := p.containerDeps.Credentials.SyncFromContainer(ctx, containerName, p.profile.Credentials); err != nil {
		return err
	}
	if codexAuthUsesAPIKey(p.profile.Credentials.Files[0].HostPath) {
		return ErrCodexAPIKeyAuth
	}
	return nil
}

func codexAuthUsesAPIKey(path string) bool {
	_, usesAPIKey := codexAuthMode(path)
	return usesAPIKey
}
