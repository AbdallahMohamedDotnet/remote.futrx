package stores

import (
	"context"
	"fmt"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	serviceuser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/user"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileauth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/filechat"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileproject"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileprojectaccess"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileprojectsecrets"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileusers"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileusersettings"
)

type AuthStore interface {
	serviceauth.Store
	OAuthConfig(context.Context) (serviceauth.OAuthConfig, error)
	SessionKey(context.Context) ([]byte, error)
}

type Stores struct {
	Chats          servicechat.Repository
	Projects       serviceproject.Repository
	ProjectSecrets serviceproject.SecretsRepository
	ProjectAccess  serviceproject.AccessRepository
	Auth           AuthStore
	Users          serviceuser.Repository
	UserSettings   serviceusersettings.Repository
}

func New(dataDir string) (Stores, error) {
	chats, err := filechat.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init chat store: %w", err)
	}

	projects, err := fileproject.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init project store: %w", err)
	}

	projectSecrets, err := fileprojectsecrets.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init project secrets store: %w", err)
	}

	projectAccess, err := fileprojectaccess.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init project access store: %w", err)
	}

	users, err := fileusers.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init users store: %w", err)
	}

	userSettings, err := fileusersettings.New(dataDir)
	if err != nil {
		return Stores{}, fmt.Errorf("init user settings store: %w", err)
	}

	return Stores{
		Chats:          chats,
		Projects:       projects,
		ProjectSecrets: projectSecrets,
		ProjectAccess:  projectAccess,
		Auth:           fileauth.New(dataDir),
		Users:          users,
		UserSettings:   userSettings,
	}, nil
}
