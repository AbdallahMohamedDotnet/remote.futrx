package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/googleoauth"
	agentauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/agent/auth"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/runhub"
	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	servicetmux "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/tmux"
	serviceuser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/user"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/workspacehub"
)

type AuthStore interface {
	serviceauth.Store
	OAuthConfig(context.Context) (serviceauth.OAuthConfig, error)
	SessionKey(context.Context) ([]byte, error)
}

type TmuxClient interface {
	servicetmux.SessionClient
}

type Dependencies struct {
	Chats             servicechat.Repository
	Projects          serviceproject.Repository
	ProjectSecrets    serviceproject.SecretsRepository
	ProjectAccess     serviceproject.AccessRepository
	Auth              AuthStore
	Users             serviceuser.Repository
	UserSettings      serviceusersettings.Repository
	AuthBaseURL       string
	ProjectContainers serviceproject.ContainerDependencies
	AgentContainers   provisioning.ContainerDependencies
	TmuxClient        TmuxClient
	ValidTmuxName     func(string) bool
}

type Services struct {
	Chats        *servicechat.Service
	ChatAccess   *servicechat.AccessService
	Projects     *serviceproject.Service
	Prompt       *prompt.Service
	AgentAuth    *agentauth.Registry
	Runs         *runhub.Hub
	Workspace    *workspacehub.Hub
	Auth         *serviceauth.Service
	Users        *serviceuser.Service
	UserSettings *serviceusersettings.Service
	Skills       *serviceskills.Catalog
	Tmux         *servicetmux.Service
	Access       *serviceauth.AccessVerifier
}

func New(ctx context.Context, deps Dependencies) (Services, error) {
	if err := deps.AgentContainers.Validate(); err != nil {
		return Services{}, fmt.Errorf("agent container dependencies: %w", err)
	}

	workspace := workspacehub.New()
	var runs *runhub.Hub
	chats := notifyingChatRepository{
		Repository: deps.Chats,
		workspace:  workspace,
		running: func(id servicechat.ID) bool {
			return runs != nil && runs.IsRunning(id)
		},
	}
	projects := notifyingProjectRepository{Repository: deps.Projects, workspace: workspace}
	definitions := agentDefinitions()
	profiles := profilesFromDefinitions(definitions)
	projectService := serviceproject.New(projects, deps.ProjectContainers, deps.ProjectSecrets, deps.ProjectAccess)
	projectService.StartAgentBrowserReaper(ctx, 20*time.Minute)
	runs = runhub.New(chats)
	runs.SetRunningSubscriber(func(id servicechat.ID, _ bool) {
		chats.publishChat(context.Background(), id)
	})

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
	chatAccessService := servicechat.NewAccessService(chatService, projectService)
	agents := agent.NewRegistry()
	agentAuth := agentauth.NewRegistry()
	for index, definition := range definitions {
		provider := definition.provider(projectService, deps.AgentContainers)
		if string(provider.ID()) != profiles[index].ID {
			return Services{}, fmt.Errorf(
				"agent registration mismatch: provider %q has profile %q",
				provider.ID(), profiles[index].ID,
			)
		}
		if err := agents.Register(provider); err != nil {
			return Services{}, err
		}
		authBinding := definition.authBinding()
		if authBinding.ID() != provider.ID() {
			return Services{}, fmt.Errorf(
				"agent auth registration mismatch: binding %q has provider %q",
				authBinding.ID(), provider.ID(),
			)
		}
		if err := agentAuth.Register(authBinding); err != nil {
			return Services{}, err
		}
	}
	promptService := prompt.New(
		chats,
		deps.TmuxClient,
		projectService,
		runs,
		agents,
	)
	userService := serviceuser.New(deps.Users)
	authService, err := newAuth(ctx, deps.Auth, userService, deps.AuthBaseURL)
	if err != nil {
		return Services{}, err
	}
	userSettingsService := serviceusersettings.New(deps.UserSettings)
	skillService := serviceskills.New()
	skillCatalog := serviceskills.NewCatalog(skillService, projectService, authService)
	var accessVerifier *serviceauth.AccessVerifier
	if authService != nil {
		accessVerifier = serviceauth.NewAccessVerifier(authService, projectService)
	}
	var tmuxService *servicetmux.Service
	if deps.TmuxClient != nil {
		tmuxService = servicetmux.NewSessions(deps.TmuxClient)
	}

	return Services{
		Chats:        chatService,
		ChatAccess:   chatAccessService,
		Projects:     projectService,
		Prompt:       promptService,
		AgentAuth:    agentAuth,
		Runs:         runs,
		Workspace:    workspace,
		Auth:         authService,
		Users:        userService,
		UserSettings: userSettingsService,
		Skills:       skillCatalog,
		Tmux:         tmuxService,
		Access:       accessVerifier,
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

// userDirectoryAdapter wraps *serviceuser.Service to satisfy
// serviceauth.UserDirectory. AddBootstrapAdmin is the one method the auth
// service needs that the regular user.Service.Add doesn't quite cover (no
// "addedBy" since it's the bootstrap path).
type userDirectoryAdapter struct {
	users *serviceuser.Service
}

func (a userDirectoryAdapter) IsAdmin(ctx context.Context, email string) (bool, error) {
	return a.users.IsAdmin(ctx, email)
}

func (a userDirectoryAdapter) IsRegistered(ctx context.Context, email string) (bool, error) {
	return a.users.IsRegistered(ctx, email)
}

func (a userDirectoryAdapter) Count(ctx context.Context) (int, error) {
	return a.users.Count(ctx)
}

func (a userDirectoryAdapter) AddBootstrapAdmin(ctx context.Context, email string) error {
	_, err := a.users.Add(ctx, email, serviceuser.RoleAdmin, "")
	return err
}

func (a userDirectoryAdapter) FirstAdmin(ctx context.Context) (*serviceauth.UserDirectoryEntry, error) {
	list, err := a.users.List(ctx)
	if err != nil {
		return nil, err
	}
	var oldest *serviceuser.User
	for i := range list {
		u := &list[i]
		if u.Role != serviceuser.RoleAdmin {
			continue
		}
		if oldest == nil || u.AddedAt < oldest.AddedAt {
			oldest = u
		}
	}
	if oldest == nil {
		return nil, nil
	}
	return &serviceauth.UserDirectoryEntry{Email: oldest.Email}, nil
}

func newAuth(
	ctx context.Context,
	store AuthStore,
	users *serviceuser.Service,
	baseURL string,
) (*serviceauth.Service, error) {
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
	var directory serviceauth.UserDirectory
	if users != nil {
		directory = userDirectoryAdapter{users: users}
	}
	return serviceauth.New(store, directory, oauthClient, baseURL, sessionKey)
}

type chatProjectResolver struct {
	projects *serviceproject.Service
}

func (r chatProjectResolver) WorkspaceForProject(ctx context.Context, id servicechat.ProjectID) (string, error) {
	return r.projects.WorkspaceForProject(ctx, serviceproject.ID(id))
}

type chatTmuxResolver struct {
	client    TmuxClient
	validName func(string) bool
}

func (r chatTmuxResolver) ValidName(name string) bool {
	return r.validName != nil && r.validName(name)
}

func (r chatTmuxResolver) Cwd(ctx context.Context, session string) (string, error) {
	return r.client.Cwd(session)
}
