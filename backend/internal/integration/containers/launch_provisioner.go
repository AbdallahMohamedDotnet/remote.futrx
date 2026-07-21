package containers

import "context"

type registeredCredentialEnsurer interface {
	EnsureRegistered(ctx context.Context, containerName string) error
}

type browserLaunchProvisioner interface {
	EnsureScript(ctx context.Context, containerName string) error
	EnsureSkill(ctx context.Context, containerName string) error
	EnsureLimits(ctx context.Context, containerName string) error
}

// containerLaunchProvisioner applies launch-time capabilities in their stable
// order. Every step is deliberately best-effort so one unavailable capability
// cannot block the remaining migrations or the newly launched container.
type containerLaunchProvisioner struct {
	credentials registeredCredentialEnsurer
	workspace   *workspaceProvisioner
	browser     browserLaunchProvisioner
	codeServer  *codeServerProvisioner
}

func (p *containerLaunchProvisioner) provision(ctx context.Context, containerName, displayName string) {
	_ = p.credentials.EnsureRegistered(ctx, containerName)
	_ = p.workspace.ensureSkillLinks(ctx, containerName)
	_ = p.browser.EnsureScript(ctx, containerName)
	_ = p.browser.EnsureSkill(ctx, containerName)
	_ = p.browser.EnsureLimits(ctx, containerName)
	_ = p.codeServer.ensure(ctx, containerName, displayName)
}
