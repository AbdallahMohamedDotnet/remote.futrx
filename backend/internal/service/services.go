package service

import (
	"context"
	"errors"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/googleoauth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
)

type AuthStore interface {
	serviceauth.Store
	OAuthConfig(context.Context) (serviceauth.OAuthConfig, error)
	SessionKey(context.Context) ([]byte, error)
}

type TmuxCwdClient interface {
	Cwd(session string) (string, error)
}

type ContainerManager interface {
	serviceproject.ContainerManager
	prompt.ContainerPreparer
}

type Dependencies struct {
	Chats         servicechat.Repository
	Projects      serviceproject.Repository
	Auth          AuthStore
	AuthBaseURL   string
	Containers    ContainerManager
	TmuxClient    TmuxCwdClient
	ValidTmuxName func(string) bool
}

type Services struct {
	Chats    *servicechat.Service
	Projects *serviceproject.Service
	Prompt   *prompt.Service
	Runs     *runhub.Hub
	Auth     *serviceauth.Service
}

func New(ctx context.Context, deps Dependencies) (Services, error) {
	projectService := serviceproject.New(deps.Projects, deps.Containers)
	runs := runhub.New(deps.Chats)

	var tmuxResolver servicechat.TmuxResolver
	if deps.TmuxClient != nil {
		tmuxResolver = chatTmuxResolver{client: deps.TmuxClient, validName: deps.ValidTmuxName}
	}

	chatService := servicechat.New(
		deps.Chats,
		chatProjectResolver{projects: projectService},
		tmuxResolver,
		runs,
	)
	promptService := prompt.New(
		deps.Chats,
		deps.TmuxClient,
		projectService,
		deps.Containers,
		runs,
	)
	authService, err := newAuth(ctx, deps.Auth, deps.AuthBaseURL)
	if err != nil {
		return Services{}, err
	}

	return Services{
		Chats:    chatService,
		Projects: projectService,
		Prompt:   promptService,
		Runs:     runs,
		Auth:     authService,
	}, nil
}

func (s Services) AuthEnabled() bool {
	return s.Auth != nil
}

func (s Services) Reconcile(ctx context.Context) error {
	if s.Projects == nil {
		return nil
	}
	return s.Projects.Reconcile(ctx)
}

func newAuth(ctx context.Context, store AuthStore, baseURL string) (*serviceauth.Service, error) {
	if store == nil {
		return nil, nil
	}
	oauthConfig, err := store.OAuthConfig(ctx)
	if err != nil {
		if errors.Is(err, serviceauth.ErrOAuthConfigNotFound) {
			return nil, nil
		}
		return nil, err
	}

	baseURL, err = serviceauth.NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	sessionKey, err := store.SessionKey(ctx)
	if err != nil {
		return nil, err
	}

	oauthClient := googleoauth.New(
		oauthConfig.GoogleClientID,
		oauthConfig.GoogleClientSecret,
		baseURL+"/auth/google/callback",
	)
	return serviceauth.New(store, oauthClient, baseURL, sessionKey)
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
