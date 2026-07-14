package project

import "context"

type Repository interface {
	List(ctx context.Context) ([]Meta, error)
	Create(ctx context.Context, meta Meta) (Meta, error)
	Get(ctx context.Context, id ID) (Meta, error)
	GetBySlug(ctx context.Context, slug string) (Meta, error)
	Update(ctx context.Context, id ID, fn func(*Meta)) (Meta, error)
	SetStatus(ctx context.Context, id ID, status Status, errMsg string) (Meta, error)
	Delete(ctx context.Context, id ID) error
}

type ContainerManager interface {
	Available() bool
	Launch(ctx context.Context, p Meta) error
	Start(ctx context.Context, containerName string) error
	Stop(ctx context.Context, containerName string) error
	Delete(ctx context.Context, containerName string) error
	State(ctx context.Context, containerName string) (ContainerState, error)
	Inspect(ctx context.Context, containerName string) (ContainerInspect, error)
	// RepairNetwork re-runs eth0 configuration (DHCP) inside the container --
	// recovers the "RUNNING but no IPv4" state systemd-networkd leaves after
	// rtnl timeouts under host load.
	RepairNetwork(ctx context.Context, containerName string) error
	ListListeners(ctx context.Context, containerName string) ([]ContainerApp, error)
	// ApplyContainerEnvDiff sets / unsets LXD environment.<KEY> entries on the
	// container so subsequent `lxc exec` sessions inherit the vars. Used by
	// the project-secrets flow to ship per-project tokens (Cloudflare, GitHub,
	// etc.) into the project's container.
	ApplyContainerEnvDiff(ctx context.Context, containerName string, set map[string]string, unset []string) error
	// EnsureAgentBrowser / StopAgentBrowser bring the Agent Browser stack up
	// and down inside the container (headed Chrome on a virtual display,
	// shared over noVNC and driven by the agent over CDP).
	EnsureAgentBrowser(ctx context.Context, containerName string) error
	EnsureAgentBrowserCore(ctx context.Context, containerName string) error
	EnsureAgentBrowserView(ctx context.Context, containerName string) error
	StopAgentBrowser(ctx context.Context, containerName string) error
	StopAgentBrowserView(ctx context.Context, containerName string) error
	AgentBrowserRunning(ctx context.Context, containerName string) (bool, error)
	AgentBrowserStatus(ctx context.Context, containerName string) (AgentBrowserInfo, error)
	// AgentBrowserPort is the in-container noVNC port the stack listens on.
	AgentBrowserPort() int
}
