// remote.futrx: self-hosted Claude Code / Codex chat + terminal-PTY server.
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

	remote "github.com/futrx-com/remote.futrx.com"
	"github.com/futrx-com/remote.futrx.com/internal/config"
	"github.com/futrx-com/remote.futrx.com/internal/integration/gitcli"
	"github.com/futrx-com/remote.futrx.com/internal/integration/hostfs"
	"github.com/futrx-com/remote.futrx.com/internal/integration/hostinfo"
	"github.com/futrx-com/remote.futrx.com/internal/integration/lxc"
	"github.com/futrx-com/remote.futrx.com/internal/integration/tmuxcli"
	service "github.com/futrx-com/remote.futrx.com/internal/service"
	servicegithistory "github.com/futrx-com/remote.futrx.com/internal/service/githistory"
	serviceserverinfo "github.com/futrx-com/remote.futrx.com/internal/service/serverinfo"
	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
	serviceworkspaceide "github.com/futrx-com/remote.futrx.com/internal/service/workspaceide"
	"github.com/futrx-com/remote.futrx.com/internal/stores"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileproject"
	"github.com/futrx-com/remote.futrx.com/internal/transport"
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
	log.Printf(
		"auth: local admin enabled; Google OAuth configured=%t; BASE_URL=%s",
		serviceSet.Auth.GoogleOAuthEnabled(),
		cfg.BaseURL,
	)
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
	codeServerBaseURL, err := config.CodeServerBaseURL(cfg.BaseURL)
	if err != nil {
		log.Fatalf("configure IDE URL: %v", err)
	}
	workspaceIDEService := serviceworkspaceide.New(codeServerBaseURL, fileproject.WorkspaceRoot)

	handler, err := transport.NewHTTPHandler(transport.Dependencies{
		Services:   serviceSet,
		TmuxClient: tmuxClient,
		Static:     static,
		DataDir:    cfg.DataDir,
		ServerInfo: serverInfoService,
		Files:      workspaceFileService,
		GitHistory: gitHistoryService,
		IDE:        workspaceIDEService,
	})
	if err != nil {
		log.Fatalf("init http handler: %v", err)
	}

	srv := transport.NewHTTPServer(cfg.Addr(), handler)
	log.Printf("remote.futrx listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
