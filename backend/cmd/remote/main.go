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
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	remote "github.com/Kings-Of-The-Web/remote.futrx.dev"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/config"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/lxc"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/tmuxcli"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/containers"
	service "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	storeSet, err := stores.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init stores: %v", err)
	}
	containerManager := containers.New(lxc.New())
	containerManager.RegisterAuthBundle(containers.ClaudeAuthBundle())
	tmuxClient := tmuxcli.New()
	serviceSet, err := service.New(ctx, service.Dependencies{
		Chats:          storeSet.Chats,
		Projects:       storeSet.Projects,
		ProjectSecrets: storeSet.ProjectSecrets,
		Auth:           storeSet.Auth,
		UserSettings:  storeSet.UserSettings,
		AuthBaseURL:    cfg.BaseURL,
		Containers:     containerManager,
		TmuxClient:     tmuxClient,
		ValidTmuxName:  tmuxcli.ValidName,
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

	// Wire the container manager's secrets source. The callback reads the
	// admin's user-settings on demand so every container ApplyContainerEnvDiff
	// (called from Launch, and from the settings PATCH handler) sees the
	// current values without a backend restart.
	adminKey := resolveAdminKey(cfg.DataDir)
	containerManager.RegisterSecretsSource(func() map[string]string {
		settings, err := serviceSet.UserSettings.Get(context.Background(), adminKey)
		if err != nil {
			log.Printf("user-settings: read secrets snapshot: %v", err)
			return nil
		}
		return settings.Secrets
	})

	static, err := fs.Sub(remote.PublicFS, "public")
	if err != nil {
		log.Fatal(err)
	}

	handler := transport.NewHTTPHandler(transport.Dependencies{
		Services:   serviceSet,
		TmuxClient: tmuxClient,
		Static:     static,
	})

	srv := transport.NewHTTPServer(cfg.Addr(), handler)
	log.Printf("remote.futrx.dev listening on %s", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// resolveAdminKey returns the user-settings key for the single admin on this
// box. With Google OAuth on, that's the sub recorded in data/admin.json
// (written the first time someone signs in and claims the box). Without
// OAuth, the LocalAdminKey is the only place settings can live.
func resolveAdminKey(dataDir string) serviceusersettings.Key {
	data, err := os.ReadFile(filepath.Join(dataDir, "admin.json"))
	if err != nil {
		return serviceusersettings.LocalAdminKey
	}
	var a struct {
		Email string `json:"email"`
		Sub   string `json:"sub"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return serviceusersettings.LocalAdminKey
	}
	key, err := serviceusersettings.KeyFromSession(a.Email, a.Sub)
	if err != nil {
		return serviceusersettings.LocalAdminKey
	}
	return key
}
