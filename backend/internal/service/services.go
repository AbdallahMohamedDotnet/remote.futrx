package service

import (
	"context"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type TmuxCwdClient interface {
	Cwd(session string) (string, error)
}

type Dependencies struct {
	Chats         servicechat.Repository
	Projects      serviceproject.Repository
	Containers    serviceproject.ContainerManager
	TmuxClient    TmuxCwdClient
	ValidTmuxName func(string) bool
	Runs          servicechat.RunController
}

type Services struct {
	Chats    *servicechat.Service
	Projects *serviceproject.Service
}

func New(deps Dependencies) Services {
	projectService := serviceproject.New(deps.Projects, deps.Containers)
	var tmuxResolver servicechat.TmuxResolver
	if deps.TmuxClient != nil {
		tmuxResolver = chatTmuxResolver{client: deps.TmuxClient, validName: deps.ValidTmuxName}
	}

	return Services{
		Chats: servicechat.New(
			deps.Chats,
			chatProjectResolver{projects: projectService},
			tmuxResolver,
			deps.Runs,
		),
		Projects: projectService,
	}
}

func (s Services) Reconcile(ctx context.Context) error {
	if s.Projects == nil {
		return nil
	}
	return s.Projects.Reconcile(ctx)
}

type chatProjectResolver struct {
	projects *serviceproject.Service
}

func (r chatProjectResolver) WorkspaceForProject(ctx context.Context, id servicechat.ProjectID) (string, error) {
	return r.projects.WorkspaceForProject(ctx, serviceproject.ID(id))
}

type chatTmuxResolver struct {
	client    TmuxCwdClient
	validName func(string) bool
}

func (r chatTmuxResolver) ValidName(name string) bool {
	return r.validName != nil && r.validName(name)
}

func (r chatTmuxResolver) Cwd(ctx context.Context, session string) (string, error) {
	return r.client.Cwd(session)
}
