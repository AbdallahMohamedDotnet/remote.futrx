// remote.futrx.dev: self-hosted Claude Code chat + terminal-PTY server.
//
// Backend serves:
//   - Static SPA (Preact/Vite bundle) embedded via go:embed
//   - HTTP API for chat metadata + per-chat upload
//   - WS /ws for tmux PTY streaming (terminal SSH bridge, no UI surfaces it)
//   - WS /ws/chat/{id} for claude streaming (stream-json + partial messages)

package main

import (
	"errors"
	"io/fs"
	"log"
	"net/http"

	remote "github.com/Kings-Of-The-Web/remote.futrx.dev"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/auth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/chat"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/claude"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/config"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/projects"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/tmux"
)

func main() {
	cfg := config.Load()

	chatStore, err := chat.NewChatStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("init chat store: %v", err)
	}

	projectStore, err := projects.NewStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("init project store: %v", err)
	}

	// Auth is optional. If data/oauth.json is absent, auth is nil and the
	// server runs open (matches old behavior for existing deployments).
	authService, err := auth.LoadAuthService(cfg.DataDir, cfg.BaseURL)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	if authService != nil {
		log.Printf("auth: Google OAuth enabled; BASE_URL=%s", cfg.BaseURL)
	} else {
		log.Printf("auth: DISABLED (no data/oauth.json) — server is open to anyone who can reach it")
	}

	static, err := fs.Sub(remote.PublicFS, "public")
	if err != nil {
		log.Fatal(err)
	}

	tmuxClient := tmux.NewClient()
	chatHandler := chat.NewHandler(chatStore, tmuxClient, projectStore)
	claudeRunner := claude.NewRunner(chatStore, tmuxClient, projectStore)
	claudeLogin := claude.NewClaudeLogin()
	projectHandler := projects.NewHandler(projectStore)
	upgrader := httpserver.NewUpgrader()

	var authRoutes *httpserver.AuthRoutes
	if authService != nil {
		routes := authService.Routes()
		authRoutes = &routes
	}

	handler := httpserver.NewHandler(httpserver.Routes{
		Sessions:        tmuxClient.HandleSessionsCollection,
		SessionResource: tmuxClient.HandleSessionResource,
		Chats:           chatHandler.HandleChatsCollection,
		ChatResource:    chatHandler.HandleChatResource,
		Projects:        projectHandler.HandleCollection,
		ProjectResource: projectHandler.HandleResource,
		ClaudeAuth:      claudeLogin.HandleStatus,
		ClaudeLogin:     claudeLogin.HandleStart,
		ClaudeCode:      claudeLogin.HandleCode,
		ClaudeCancel:    claudeLogin.HandleCancel,
		TmuxWS:          tmuxClient.PTYHandler(upgrader),
		ChatWS:          claudeRunner.StreamHandler(upgrader),
		Auth:            authRoutes,
		Static:          http.FileServer(http.FS(static)),
	})

	srv := httpserver.NewServer(cfg.Addr(), handler)
	log.Printf("remote.futrx.dev listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
