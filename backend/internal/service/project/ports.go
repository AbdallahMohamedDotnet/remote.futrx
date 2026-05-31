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
}
