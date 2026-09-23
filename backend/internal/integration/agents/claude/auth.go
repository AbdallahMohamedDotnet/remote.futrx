package claude

import (
	"context"
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
	claudeLoginTimeout = 10 * time.Minute
	claudeURLReadWait  = 15 * time.Second
	claudeExitWait     = 30 * time.Second
)

var (
	ErrCodeRequired   = errors.New("code is required")
	ErrNoSession      = errors.New("no login session in progress - call /api/claude/login/start first")
	ErrClaudeNotFound = errors.New("claude CLI not found on PATH - install it first")

	claudeAuthURLRE = regexp.MustCompile(`https://claude\.com/cai/oauth/authorize\?[^\s]+`)
)

type AuthStatus = agentauth.CodeStatus
type LoginState = agentauth.CodeLoginState
type StartResult = agentauth.CodeStartResult

// Auth combines Claude's authorization-code login with its optional
// saved-account vault. CodeService continues to own the login process and
// subscriptions; Auth owns only Claude credential placement and account
// selection.
type Auth struct {
	code  *agentauth.CodeService
	store agentauth.AccountStore

	mutationMu sync.Mutex
	mu         sync.Mutex
	accounts   agentauth.AccountSet
	attempt    *accountLoginAttempt
	activeRuns int
	validate   func(context.Context, []byte) (validatedAccount, error)
}

// NewAuth configures the shared authorization-code service for the Claude CLI
// and restores the saved active account, when one exists.
func NewAuth(store agentauth.AccountStore) (*Auth, error) {
	auth := &Auth{store: store, validate: validateAccountCredential}
	if store != nil {
		accounts, err := store.AgentAccounts(context.Background(), agent.ProviderClaude)
		if err != nil {
			return nil, err
		}
		auth.accounts = accounts
		if err := auth.restoreActiveAccount(); err != nil {
			return nil, fmt.Errorf("restore active Claude account: %w", err)
		}
	}
	auth.code = agentauth.NewCodeService(agentauth.CodeConfig{
		Command:           "claude",
		Args:              []string{"auth", "login", "--claudeai"},
		Env:               auth.codeAuthEnv,
		ResolveCompletion: auth.resolveCompletion,
		URLPattern:        claudeAuthURLRE,
		LoginTimeout:      claudeLoginTimeout,
		URLReadTimeout:    claudeURLReadWait,
		ExitTimeout:       claudeExitWait,
		Authenticated:     authenticated,
		NotFound:          ErrClaudeNotFound,
		CodeRequired:      ErrCodeRequired,
		NoSession:         ErrNoSession,
		Errors: agentauth.CodeErrorFormatters{
			PTYStart: func(err error) error {
				return fmt.Errorf("pty start: %w", err)
			},
			URLReadTimeout: func(wait time.Duration, output string) error {
				return fmt.Errorf(
					"did not see Anthropic OAuth URL within %s; first 500 bytes of claude output: %s",
					wait,
					output,
				)
			},
			ExitBeforeURL: func(err error, output string) error {
				message := "claude exited before printing OAuth URL"
				if err != nil {
					message += " (" + err.Error() + ")"
				}
				return fmt.Errorf("%s; output: %s", message, output)
			},
			WriteCode: func(err error) error {
				return fmt.Errorf("write to claude stdin: %w", err)
			},
			ExitTimeout: func(wait time.Duration, output string) error {
				return fmt.Errorf(
					"claude did not exit within %s after code paste; last output: %s",
					wait,
					output,
				)
			},
			Exit: func(err error, output string) error {
				return fmt.Errorf("claude exited with error: %w; output: %s", err, output)
			},
			MissingCredentials: func(output string) error {
				return fmt.Errorf(
					"claude exited cleanly but no credentials file was written; output: %s",
					output,
				)
			},
		},
	})
	return auth, nil
}

func (a *Auth) Authenticated() bool                    { return a.code.Authenticated() }
func (a *Auth) Status() AuthStatus                     { return a.code.Status() }
func (a *Auth) Subscribe() (<-chan AuthStatus, func()) { return a.code.Subscribe() }
func (a *Auth) Broadcast()                             { a.code.Broadcast() }
func (a *Auth) Start(ctx context.Context) (StartResult, error) {
	return a.code.Start(ctx)
}
func (a *Auth) SubmitCode(ctx context.Context, code string) error {
	return a.code.SubmitCode(ctx, code)
}
func (a *Auth) Cancel(ctx context.Context) error { return a.code.Cancel(ctx) }

func authenticated() bool {
	for _, name := range []string{".credentials.json", "credentials.json"} {
		if credentialUsable(filepath.Join(claudeHomeDir(), name)) {
			return true
		}
	}
	return false
}

// credentialUsable reports whether path holds a Claude subscription login the
// CLI can still use, rather than merely existing: a stale or expired
// credentials file with no refresh token must not read as signed in.
func credentialUsable(path string) bool {
	credentials, err := readJSONObject(path)
	if err != nil {
		return false
	}
	oauth, ok := credentials["claudeAiOauth"]
	if !ok || len(oauth) == 0 {
		return false
	}
	return checkOAuthUsable(oauth, time.Now()) == nil
}

func claudeHomeDir() string {
	if value := os.Getenv("CLAUDE_CONFIG_DIR"); value != "" {
		return value
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".claude")
	}
	return "/root/.claude"
}

func claudeCredentialPath() string {
	return filepath.Join(claudeHomeDir(), ".credentials.json")
}

// claudeGlobalConfigPath is where the CLI keeps oauthAccount. It lives inside
// CLAUDE_CONFIG_DIR when that is set and beside the home directory otherwise.
func claudeGlobalConfigPath() string {
	if value := os.Getenv("CLAUDE_CONFIG_DIR"); value != "" {
		return filepath.Join(value, ".claude.json")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".claude.json")
	}
	return "/root/.claude.json"
}

// isolatedClaudeAuthEnvFor points the CLI at a private config directory and
// removes inherited credentials that would take precedence over the
// subscription login being created or inspected. The CLI stores OAuth tokens
// under CLAUDE_SECURESTORAGE_CONFIG_DIR when that is set, so it is removed
// too; otherwise a login would land outside the private directory.
func isolatedClaudeAuthEnvFor(base []string, configDir string) []string {
	out := make([]string, 0, len(base)+1)
	for _, env := range base {
		if strings.HasPrefix(env, "CLAUDE_CONFIG_DIR=") ||
			strings.HasPrefix(env, "CLAUDE_SECURESTORAGE_CONFIG_DIR=") ||
			strings.HasPrefix(env, "ANTHROPIC_API_KEY=") ||
			strings.HasPrefix(env, "ANTHROPIC_AUTH_TOKEN=") ||
			strings.HasPrefix(env, "CLAUDE_CODE_OAUTH_TOKEN=") {
			continue
		}
		out = append(out, env)
	}
	return append(out, "CLAUDE_CONFIG_DIR="+configDir)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
