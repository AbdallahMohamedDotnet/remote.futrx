package stores

import (
	"context"
	"fmt"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileauth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/filechat"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileproject"
)

type AuthStore interface {
	serviceauth.Store
	OAuthConfig(context.Context) (serviceauth.OAuthConfig, error)
	SessionKey(context.Context) ([]byte, error)
}

type Stores struct {
	Chats    servicechat.Repository
	Projects serviceproject.Repository
	Auth     AuthStore
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

	return Stores{
		Chats:    chats,
		Projects: projects,
		Auth:     fileauth.New(dataDir),
	}, nil
}
