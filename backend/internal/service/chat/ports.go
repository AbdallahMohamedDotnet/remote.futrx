package chat

import "context"

type Repository interface {
	List(ctx context.Context) ([]Meta, error)
	Create(ctx context.Context, meta Meta) (Meta, error)
	Get(ctx context.Context, id ID) (Meta, error)
	Update(ctx context.Context, id ID, fn func(*Meta)) (Meta, error)
	Delete(ctx context.Context, id ID) error

	ReadEvents(ctx context.Context, id ID) ([]Event, error)
	AppendEvent(ctx context.Context, id ID, ev Event) error
	TruncateEventsBefore(ctx context.Context, id ID, beforeT int64) ([]Event, error)
}

type ProjectResolver interface {
	WorkspaceForProject(ctx context.Context, id ProjectID) (string, error)
}

type TmuxResolver interface {
	ValidName(name string) bool
	Cwd(ctx context.Context, session string) (string, error)
}

type RunController interface {
	IsRunning(id ID) bool
	Cancel(ctx context.Context, id ID) error
}
