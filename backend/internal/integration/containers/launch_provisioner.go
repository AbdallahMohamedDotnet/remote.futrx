package containers

import "context"

// containerLaunchProvisioner applies launch-time capabilities in their stable
// order. Every step is deliberately best-effort so one unavailable capability
// cannot block the remaining migrations or the newly launched container.
type containerLaunchProvisioner struct {
	credentials *credentialSynchronizer
	workspace   *workspaceProvisioner
	browser     *agentBrowserConfigurator
	codeServer  *codeServerProvisioner
}

func (p *containerLaunchProvisioner) provision(ctx context.Context, containerName, displayName string) {
	_ = p.credentials.ensureRegistered(ctx, containerName)
	_ = p.workspace.ensureSkillLinks(ctx, containerName)
	_ = p.workspace.ensureBrowserScript(ctx, containerName)
	_ = p.workspace.ensureBrowserSkill(ctx, containerName)
	_ = p.browser.ensure(ctx, containerName)
	_ = p.codeServer.ensure(ctx, containerName, displayName)
}
