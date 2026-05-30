// remote.futrx.dev: self-hosted Claude Code chat + terminal-PTY server.
//
// Backend serves:
//   - Static SPA (Preact/Vite bundle) embedded via go:embed
//   - HTTP API for chat metadata + per-chat upload
//   - WS /ws for tmux PTY streaming (terminal SSH bridge, no UI surfaces it)
//   - WS /ws/chat/{id} for claude streaming (stream-json + partial messages)

package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"

	remote "github.com/Kings-Of-The-Web/remote.futrx.dev"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/config"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/lxc"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/tmuxcli"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	service "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport"
)

func main() {
	cfg := config.Load()

	storeSet, err := stores.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init stores: %v", err)
	}
	containerManager := lxc.New()

	// Auth is optional. If data/oauth.json is absent, authHandler is nil and the
	// server runs open (matches old behavior for existing deployments).
	authHandler, authEnabled, err := loadAuthHandler(context.Background(), storeSet.Auth, cfg.BaseURL)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	if authEnabled {
		log.Printf("auth: Google OAuth enabled; BASE_URL=%s", cfg.BaseURL)
	} else {
		log.Printf("auth: DISABLED (no data/oauth.json) — server is open to anyone who can reach it")
	}

	static, err := fs.Sub(remote.PublicFS, "public")
	if err != nil {
		log.Fatal(err)
	}

	tmuxClient := tmuxcli.New()
	runHub := runhub.New(storeSet.Chats)
	serviceSet := service.New(service.Dependencies{
		Chats:         storeSet.Chats,
		Projects:      storeSet.Projects,
		Containers:    containerManager,
		TmuxClient:    tmuxClient,
		ValidTmuxName: tmuxcli.ValidName,
		Runs:          runHub,
	})
	if err := serviceSet.Reconcile(context.Background()); err != nil {
		log.Printf("services: reconcile warning: %v", err)
	}

	handler := transport.NewHTTPHandler(transport.Dependencies{
		ChatStore:        storeSet.Chats,
		ChatService:      serviceSet.Chats,
		ProjectService:   serviceSet.Projects,
		TmuxClient:       tmuxClient,
		RunHub:           runHub,
		ContainerManager: containerManager,
		Auth:             authHandler,
		Static:           static,
	})

	srv := transport.NewHTTPServer(cfg.Addr(), handler)
	log.Printf("remote.futrx.dev listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
