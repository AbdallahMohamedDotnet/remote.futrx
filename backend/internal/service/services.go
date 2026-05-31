package service

import (
	"context"
	"errors"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/googleoauth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/workspacehub"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
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
	// ApplyContainerEnvDiff is used by the user-settings flow to push global
	// secrets (Cloudflare/GitHub/etc. tokens) into a project container.
	ApplyContainerEnvDiff(ctx context.Context, container string, set map[string]string, unset []string) error
}

type Dependencies struct {
	Chats          servicechat.Repository
	Projects       serviceproject.Repository
	ProjectSecrets serviceproject.SecretsRepository
	Auth           AuthStore
	UserSettings   serviceusersettings.Repository
	AuthBaseURL    string
	Containers     ContainerManager
	TmuxClient     TmuxCwdClient
	ValidTmuxName  func(string) bool
}

type Services struct {
	Chats        *servicechat.Service
	Projects     *serviceproject.Service
	Prompt       *prompt.Service
	Runs         *runhub.Hub
	Workspace    *workspacehub.Hub
	Auth         *serviceauth.Service
	UserSettings *serviceusersettings.Service
	// Containers is the underlying container manager surfaced here so HTTP
	// handlers that need to push container-level state (e.g. global secrets
	// from the user-settings flow) can reach it without a second wiring path.
	Containers   ContainerManager
}

func New(ctx context.Context, deps Dependencies) (Services, error) {
	workspace := workspacehub.New()
	chats := notifyingChatRepository{Repository: deps.Chats, workspace: workspace}
	projects := notifyingProjectRepository{Repository: deps.Projects, workspace: workspace}
	projectService := serviceproject.New(projects, deps.Containers, deps.ProjectSecrets)
	runs := runhub.New(chats)

	var tmuxResolver servicechat.TmuxResolver
	if deps.TmuxClient != nil {
		tmuxResolver = chatTmuxResolver{client: deps.TmuxClient, validName: deps.ValidTmuxName}
	}

	chatService := servicechat.New(
		chats,
		chatProjectResolver{projects: projectService},
		tmuxResolver,
		runs,
	)
	promptService := prompt.New(
		chats,
		deps.TmuxClient,
		projectService,
		deps.Containers,
		runs,
	)
	authService, err := newAuth(ctx, deps.Auth, deps.AuthBaseURL)
	if err != nil {
		return Services{}, err
	}
	userSettingsService := serviceusersettings.New(deps.UserSettings)

	return Services{
		Chats:        chatService,
		Projects:     projectService,
		Prompt:       promptService,
		Runs:         runs,
		Workspace:    workspace,
		Auth:         authService,
		UserSettings: userSettingsService,
		Containers:   deps.Containers,
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
