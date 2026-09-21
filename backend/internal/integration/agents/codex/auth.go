package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

const (
	deviceLoginReadyTimeout = 8 * time.Second
	deviceLoginTimeout      = 16 * time.Minute
	deviceLoginTTL          = 15 * time.Minute
)

var (
	ErrCodexNotFound = errors.New("codex CLI not found on PATH - install it first")

	deviceURLRE  = regexp.MustCompile(`https://auth\.openai\.com/codex/device`)
	deviceCodeRE = regexp.MustCompile(`[A-Z0-9]{4}-[A-Z0-9]{5}`)
)

type AuthStatus struct {
	Authenticated bool             `json:"authenticated"`
	AuthMode      string           `json:"authMode,omitempty"`
	UsesAPIKey    bool             `json:"usesApiKey,omitempty"`
	DeviceLogin   DeviceLoginState `json:"deviceLogin,omitempty"`
}

type DeviceLoginState = agentauth.DeviceState

// Auth combines Codex's device login with its optional saved-account vault.
// DeviceService continues to own process lifecycle and subscriptions; Auth
// owns only Codex credential placement and account selection.
type Auth struct {
	device *agentauth.DeviceService[AuthStatus]
	store  agentauth.AccountStore

	mutationMu sync.Mutex
	mu         sync.Mutex
	accounts   agentauth.AccountSet
	attempt    *accountLoginAttempt
	activeRuns int
	validate   func(context.Context, []byte) (validatedAccount, error)
}

func NewAuth(store agentauth.AccountStore) (*Auth, error) {
	auth := &Auth{store: store, validate: validateAccountCredential}
	if store != nil {
		accounts, err := store.AgentAccounts(context.Background(), agent.ProviderCodex)
		if err != nil {
			return nil, err
		}
		auth.accounts = accounts
	}
	auth.device = agentauth.NewDeviceService(agentauth.DeviceConfig[AuthStatus]{
		Command:         "codex",
		Args:            []string{"login", "--device-auth"},
		Env:             auth.deviceAuthEnv,
		NotFound:        ErrCodexNotFound,
		StartErrorLabel: "codex login",
		ReadyTimeout:    deviceLoginReadyTimeout,
		LoginTimeout:    deviceLoginTimeout,
		LoginTTL:        deviceLoginTTL,
		URLPattern:      deviceURLRE,
		CodePattern:     deviceCodeRE,
		Authenticated: func() bool {
			authenticated, _, _ := authenticated()
			return authenticated
		},
		BuildStatus: func() agentauth.DeviceStatusBuilder[AuthStatus] {
			authenticated, authMode, usesAPIKey := authenticated()
			return func(state agentauth.DeviceState) AuthStatus {
				return AuthStatus{
					Authenticated: authenticated,
					AuthMode:      authMode,
					UsesAPIKey:    usesAPIKey,
					DeviceLogin:   state,
				}
			}
		},
		ResolveCompletion: func(err error) agentauth.DeviceCompletion {
			if auth.hasAccountAttempt() {
				return auth.completeAccountLogin(err)
			}
			authenticated, _, usesAPIKey := authenticated()
			switch {
			case authenticated:
				return agentauth.DeviceCompletion{Completed: true}
			case usesAPIKey:
				return agentauth.DeviceCompletion{Error: "Codex is still logged in with an API key. Sign in with ChatGPT to use subscription limits."}
			case err != nil:
				return agentauth.DeviceCompletion{Error: fmt.Sprintf("codex login failed: %s", truncate(err.Error(), 300))}
			default:
				return agentauth.DeviceCompletion{Error: "Codex login ended before authentication completed."}
			}
		},
	})
	return auth, nil
}

func (a *Auth) Authenticated() bool                    { return a.device.Authenticated() }
func (a *Auth) Status() AuthStatus                     { return a.device.Status() }
func (a *Auth) LoginState() agentauth.DeviceState      { return a.device.LoginState() }
func (a *Auth) Subscribe() (<-chan AuthStatus, func()) { return a.device.Subscribe() }
func (a *Auth) StartDeviceLogin(ctx context.Context) (agentauth.DeviceState, error) {
	return a.device.StartDeviceLogin(ctx)
}

func (a *Auth) Broadcast() { a.device.Broadcast() }

func authenticated() (bool, string, bool) {
	authPath := filepath.Join(codexHomeDir(), "auth.json")
	authMode, usesAPIKey := codexAuthMode(authPath)
	if usesAPIKey {
		return false, authMode, true
	}
	if authMode == "" {
		return false, "", false
	}
	return true, authMode, false
}

func codexAuthMode(authPath string) (string, bool) {
	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "unknown", false
	}
	mode, _ := raw["auth_mode"].(string)
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		if _, hasAPIKey := raw["OPENAI_API_KEY"]; hasAPIKey {
			return "apikey", true
		}
		return "unknown", false
	}
	return mode, mode == "apikey"
}

func codexHomeDir() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return filepath.Join(v, ".codex")
	}
	return "/root/.codex"
}

func codexAuthEnv(base []string) []string {
	return codexAuthEnvFor(base, codexHomeDir())
}

func codexAuthEnvFor(base []string, home string) []string {
	out := make([]string, 0, len(base)+1)
	for _, env := range base {
		if strings.HasPrefix(env, "OPENAI_API_KEY=") {
			continue
		}
		if strings.HasPrefix(env, "CODEX_HOME=") {
			continue
		}
		out = append(out, env)
	}
	return append(out, "CODEX_HOME="+home)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
