// remote.futrx.dev: self-hosted Claude Code / Codex chat + terminal-PTY server.
//
// Backend serves:
//   - Static SPA (Preact/Vite bundle) embedded via go:embed
//   - HTTP API for chat metadata + per-chat upload
//   - WS /ws for tmux PTY streaming (terminal SSH bridge, no UI surfaces it)
//   - WS /ws/chat/{id} for agent streaming

package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"

	remote "github.com/Kings-Of-The-Web/remote.futrx.dev"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/config"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/gitcli"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/hostfs"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/hostinfo"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/lxc"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/tmuxcli"
	service "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service"
	servicegithistory "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/githistory"
	serviceserverinfo "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/serverinfo"
	serviceworkspacefiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/workspacefiles"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileproject"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	storeSet, err := stores.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init stores: %v", err)
	}
	lxcClient := lxc.New()
	containerStack := config.NewContainerStack(lxcClient, service.AgentProfiles())
	tmuxClient := tmuxcli.New()
	serviceSet, err := service.New(ctx, service.Dependencies{
		Chats:             storeSet.Chats,
		Projects:          storeSet.Projects,
		ProjectSecrets:    storeSet.ProjectSecrets,
		ProjectAccess:     storeSet.ProjectAccess,
		Auth:              storeSet.Auth,
		Users:             storeSet.Users,
		UserSettings:      storeSet.UserSettings,
		AuthBaseURL:       cfg.BaseURL,
		ProjectContainers: containerStack.ProjectDependencies(),
		AgentContainers:   containerStack.AgentDependencies(),
		TmuxClient:        tmuxClient,
		ValidTmuxName:     tmuxcli.ValidName,
	})
	if err != nil {
		log.Fatalf("init services: %v", err)
	}
	if serviceSet.AuthEnabled() {
		log.Printf("auth: Google OAuth enabled; BASE_URL=%s", cfg.BaseURL)
	} else {
		log.Printf("auth: DISABLED (no data/oauth.json) — server is open to anyone who can reach it")
	}
	if err := serviceSet.Reconcile(ctx); err != nil {
		log.Printf("services: reconcile warning: %v", err)
	}

	static, err := fs.Sub(remote.PublicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	serverInfoService := serviceserverinfo.New(hostinfo.New(), cfg.DataDir, fileproject.WorkspaceRoot)
	workspaceFileService := serviceworkspacefiles.New(hostfs.NewWorkspaceFileStore())
	gitHistoryService := servicegithistory.New(gitcli.NewHistoryClient())

	handler, err := transport.NewHTTPHandler(transport.Dependencies{
		Services:   serviceSet,
		TmuxClient: tmuxClient,
		Static:     static,
		DataDir:    cfg.DataDir,
		ServerInfo: serverInfoService,
		Files:      workspaceFileService,
		GitHistory: gitHistoryService,
	})
	if err != nil {
		log.Fatalf("init http handler: %v", err)
	}

	srv := transport.NewHTTPServer(cfg.Addr(), handler)
	log.Printf("remote.futrx.dev listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
