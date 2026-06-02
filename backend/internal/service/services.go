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
	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	serviceuser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/user"
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
	Runs         *runhub.Hub
	Workspace    *workspacehub.Hub
	Auth         *serviceauth.Service
	Users        *serviceuser.Service
	UserSettings *serviceusersettings.Service
	Skills       *serviceskills.Service
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
	promptService := prompt.New(
		chats,
		deps.TmuxClient,
		projectService,
		deps.Containers,
		runs,
	)
	userService := serviceuser.New(deps.Users)
	if err := migrateLegacyAdmin(ctx, deps.Auth, userService); err != nil {
		return Services{}, err
	}
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

// migrateLegacyAdmin promotes a pre-multi-user admin.json into the new
// users.json store. Runs once on first boot after this feature lands: the
// box was already claimed by one admin, so we mirror that into users.json
// with role=admin. Subsequent boots are no-ops because the user is already
// in the store.
func migrateLegacyAdmin(ctx context.Context, store AuthStore, users *serviceuser.Service) error {
	if store == nil || users == nil {
		return nil
	}
	count, err := users.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	admin, err := store.Admin(ctx)
	if err != nil || admin == nil {
		return nil
	}
	if _, err := users.Add(ctx, admin.Email, serviceuser.RoleAdmin, ""); err != nil {
		// Tolerate any user-level error (e.g. invalid email format). The
		// bootstrap path in auth.claimOrAuthorize will reseed on next sign-in.
		return nil
	}
	return nil
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
