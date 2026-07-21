package service

import (
	"context"
	"errors"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	claudeagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/claude"
	codexagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/codex"
	kimiagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/kimi"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/googleoauth"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/runhub"
	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	serviceuser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/user"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/workspacehub"
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
	claudeagent.ContainerPreparer
	codexagent.ContainerPreparer
	kimiagent.ContainerPreparer
}

type Dependencies struct {
	Chats          servicechat.Repository
	Projects       serviceproject.Repository
	ProjectSecrets serviceproject.SecretsRepository
	ProjectAccess  serviceproject.AccessRepository
	Auth           AuthStore
	Users          serviceuser.Repository
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
	AgentAuth    AgentAuthServices
	Runs         *runhub.Hub
	Workspace    *workspacehub.Hub
	Auth         *serviceauth.Service
	Users        *serviceuser.Service
	UserSettings *serviceusersettings.Service
	Skills       *serviceskills.Service
}

// AgentAuthServices are the shared auth lifecycles configured by each
// registered agent. Provider packages supply policy; service/agent/auth owns
// the process and subscription behavior.
type AgentAuthServices struct {
	Claude *claudeagent.Auth
	Codex  *codexagent.Auth
	Kimi   *kimiagent.Auth
}

type agentRegistration struct {
	provider      agent.Provider
	configureAuth func(*AgentAuthServices)
}

func New(ctx context.Context, deps Dependencies) (Services, error) {
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
	projectService := serviceproject.New(projects, deps.Containers, deps.ProjectSecrets, deps.ProjectAccess)
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
	agents := agent.NewRegistry()
	agentAuth := AgentAuthServices{}
	for _, registration := range []agentRegistration{
		{
			provider: claudeagent.New(projectService, deps.Containers),
			configureAuth: func(auth *AgentAuthServices) {
				auth.Claude = claudeagent.NewAuth()
			},
		},
		{
			provider: codexagent.New(projectService, deps.Containers),
			configureAuth: func(auth *AgentAuthServices) {
				auth.Codex = codexagent.NewAuth()
			},
		},
		{
			provider: kimiagent.New(projectService, deps.Containers),
			configureAuth: func(auth *AgentAuthServices) {
				auth.Kimi = kimiagent.NewAuth()
			},
		},
	} {
		if err := agents.Register(registration.provider); err != nil {
			return Services{}, err
		}
		registration.configureAuth(&agentAuth)
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

	return Services{
		Chats:        chatService,
		Projects:     projectService,
		Prompt:       promptService,
		AgentAuth:    agentAuth,
		Runs:         runs,
		Workspace:    workspace,
		Auth:         authService,
		Users:        userService,
		UserSettings: userSettingsService,
		Skills:       skillService,
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
	client    TmuxCwdClient
	validName func(string) bool
}

func (r chatTmuxResolver) ValidName(name string) bool {
	return r.validName != nil && r.validName(name)
}

func (r chatTmuxResolver) Cwd(ctx context.Context, session string) (string, error) {
	return r.client.Cwd(session)
}
