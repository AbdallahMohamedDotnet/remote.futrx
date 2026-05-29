package main

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/tmuxcli"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type chatProjectResolver struct {
	projects *serviceproject.Service
}

func (r chatProjectResolver) WorkspaceForProject(ctx context.Context, id servicechat.ProjectID) (string, error) {
	return r.projects.WorkspaceForProject(ctx, serviceproject.ID(id))
}

type chatTmuxResolver struct {
	client *tmuxcli.Client
}

func (r chatTmuxResolver) ValidName(name string) bool {
	return tmuxcli.ValidName(name)
}

func (r chatTmuxResolver) Cwd(ctx context.Context, session string) (string, error) {
	return r.client.Cwd(session)
}
