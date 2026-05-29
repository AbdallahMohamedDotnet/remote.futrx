package main

import (
	"context"
	"errors"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/googleoauth"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileauth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
	httphandlers "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http/handlers"
)

func loadAuthRoutes(ctx context.Context, dataDir, baseURL string) (*httptransport.AuthRoutes, bool, error) {
	store := fileauth.New(dataDir)
	oauthConfig, err := store.OAuthConfig(ctx)
	if err != nil {
		if errors.Is(err, serviceauth.ErrOAuthConfigNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	baseURL, err = serviceauth.NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, false, err
	}
	sessionKey, err := store.SessionKey(ctx)
	if err != nil {
		return nil, false, err
	}

	oauthClient := googleoauth.New(
		oauthConfig.GoogleClientID,
		oauthConfig.GoogleClientSecret,
		baseURL+"/auth/google/callback",
	)
	authService, err := serviceauth.New(store, oauthClient, baseURL, sessionKey)
	if err != nil {
		return nil, false, err
	}
	routes := httphandlers.NewAuthHandler(authService).Routes()
	return &routes, true, nil
}
