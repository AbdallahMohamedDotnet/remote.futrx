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
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/claudelogin"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/filechat"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileproject"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
	httphandlers "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http/handlers"
	wstransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/ws"
)

func main() {
	cfg := config.Load()

	chatStore, err := filechat.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init chat store: %v", err)
	}

	projectStore, err := fileproject.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init project store: %v", err)
	}
	containerManager := lxc.New()
	projectService := serviceproject.New(projectStore, containerManager)
	if err := projectService.Reconcile(context.Background()); err != nil {
		log.Printf("projects: reconcile warning: %v", err)
	}

	// Auth is optional. If data/oauth.json is absent, authHandler is nil and the
	// server runs open (matches old behavior for existing deployments).
	authHandler, authEnabled, err := loadAuthHandler(context.Background(), cfg.DataDir, cfg.BaseURL)
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
	runHub := runhub.New(chatStore)
	chatService := servicechat.New(
		chatStore,
		chatProjectResolver{projects: projectService},
		chatTmuxResolver{client: tmuxClient},
		runHub,
	)
	chatHandler := httphandlers.NewChatHandler(chatService)
	promptService := prompt.New(chatStore, tmuxClient, projectService, containerManager, runHub)
	chatSocket := wstransport.NewChatSocket(chatStore, runHub, promptService)
	claudeLogin := claudelogin.New()
	claudeAuthHandler := httphandlers.NewClaudeAuthHandler(claudeLogin)
	projectHandler := httphandlers.NewProjectHandler(projectService)
	tmuxHandler := httphandlers.NewTmuxHandler(tmuxClient)
	tmuxSocket := wstransport.NewTmuxSocket(tmuxClient)

	handler := httptransport.NewHandler(httptransport.Handlers{
		Sessions:   tmuxHandler,
		Chats:      chatHandler,
		Projects:   projectHandler,
		ClaudeAuth: claudeAuthHandler,
		TmuxWS:     tmuxSocket,
		ChatWS:     chatSocket,
		Auth:       authHandler,
		Static:     httptransport.NewStaticHandler(static),
	})

	srv := httptransport.NewServer(cfg.Addr(), handler)
	log.Printf("remote.futrx.dev listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
