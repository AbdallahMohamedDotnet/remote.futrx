// Package launch coordinates best-effort capabilities after a new container
// is launched.
package launch

import "context"

type RegisteredCredentialEnsurer interface {
	EnsureRegistered(ctx context.Context, containerName string) error
}

type BrowserProvisioner interface {
	EnsureScript(ctx context.Context, containerName string) error
	EnsureSkill(ctx context.Context, containerName string) error
	EnsureLimits(ctx context.Context, containerName string) error
}

type WorkspaceProvisioner interface {
	EnsureSkillLinks(ctx context.Context, containerName string) error
}

type CodeServerProvisioner interface {
	Ensure(ctx context.Context, containerName, displayName string) error
}

// containerLaunchProvisioner applies launch-time capabilities in their stable
// order. Every step is deliberately best-effort so one unavailable capability
// cannot block the remaining migrations or the newly launched container.
type Provisioner struct {
	credentials RegisteredCredentialEnsurer
	workspace   WorkspaceProvisioner
	browser     BrowserProvisioner
	codeServer  CodeServerProvisioner
}

// NewProvisioner returns the stable launch-time capability sequence.
func NewProvisioner(
	credentials RegisteredCredentialEnsurer,
	workspace WorkspaceProvisioner,
	browser BrowserProvisioner,
	codeServer CodeServerProvisioner,
) *Provisioner {
	return &Provisioner{
		credentials: credentials,
		workspace:   workspace,
		browser:     browser,
		codeServer:  codeServer,
	}
}

// Provision applies launch-time capabilities in their stable order. Every
// step is deliberately best-effort so one unavailable capability cannot block
// the remaining migrations or the newly launched container.
func (p *Provisioner) Provision(ctx context.Context, containerName, displayName string) {
	_ = p.credentials.EnsureRegistered(ctx, containerName)
	_ = p.workspace.EnsureSkillLinks(ctx, containerName)
	_ = p.browser.EnsureScript(ctx, containerName)
	_ = p.browser.EnsureSkill(ctx, containerName)
	_ = p.browser.EnsureLimits(ctx, containerName)
	_ = p.codeServer.Ensure(ctx, containerName, displayName)
}
