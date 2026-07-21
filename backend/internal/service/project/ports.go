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

// ContainerLifecycle is the project service's container state-transition port.
// Implementations may use LXD or any other runtime; project policy only relies
// on these lifecycle operations.
type ContainerLifecycle interface {
	Available() bool
	Launch(ctx context.Context, p Meta) error
	Start(ctx context.Context, containerName string) error
	Stop(ctx context.Context, containerName string) error
	Delete(ctx context.Context, containerName string) error
	State(ctx context.Context, containerName string) (ContainerState, error)
	// EnsureResources converges the fleet-default resource envelope (the
	// managed LXD profile) onto an existing container.
	EnsureResources(ctx context.Context, containerName string) error
}

// ContainerEnvironment applies project secrets to future container sessions.
type ContainerEnvironment interface {
	ApplyDiff(ctx context.Context, containerName string, set map[string]string, unset []string) error
}

// ContainerInspector returns a best-effort diagnostic snapshot.
type ContainerInspector interface {
	Inspect(ctx context.Context, containerName string) (ContainerInspect, error)
}

// ContainerNetwork repairs guest network configuration.
type ContainerNetwork interface {
	Repair(ctx context.Context, containerName string) error
}

// ContainerListeners discovers externally reachable guest applications.
type ContainerListeners interface {
	List(ctx context.Context, containerName string) ([]ContainerApp, error)
}

// ContainerBrowser manages the browser capabilities consumed by projects.
type ContainerBrowser interface {
	Ensure(ctx context.Context, containerName string) error
	Stop(ctx context.Context, containerName string) error
	StopView(ctx context.Context, containerName string) error
	Status(ctx context.Context, containerName string) (AgentBrowserInfo, error)
	Port() int
}

// ContainerDependencies groups the independently replaceable container
// capabilities used by Service. A nil capability preserves the behavior of a
// nil container manager for the operations that consume it.
type ContainerDependencies struct {
	Lifecycle   ContainerLifecycle
	Environment ContainerEnvironment
	Inspector   ContainerInspector
	Network     ContainerNetwork
	Listeners   ContainerListeners
	Browser     ContainerBrowser
}
