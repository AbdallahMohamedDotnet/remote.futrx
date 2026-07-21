package kimiauth

// Service configures the host-side `kimi login` device-code flow for
// @moonshot-ai/kimi-code. Unlike Codex there is no API-key mode: Kimi Code
// auth is always a subscription OAuth grant under ~/.kimi-code/credentials/.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/deviceauth"
)

const (
	deviceLoginReadyTimeout = 8 * time.Second
	deviceLoginTimeout      = 30 * time.Minute
	deviceLoginTTL          = 29 * time.Minute
)

var (
	ErrKimiNotFound = errors.New("kimi CLI not found on PATH - install it first")

	// kimi prints e.g.
	//   https://www.kimi.com/code/authorize_device?user_code=T906-Q0QV
	deviceURLRE  = regexp.MustCompile(`https://www\.kimi\.com/code/authorize_device\S*`)
	deviceCodeRE = regexp.MustCompile(`[A-Z0-9]{4}-[A-Z0-9]{4,6}`)
)

type Status struct {
	Authenticated bool             `json:"authenticated"`
	DeviceLogin   DeviceLoginState `json:"deviceLogin,omitempty"`
}

type DeviceLoginState = deviceauth.State
type Service = deviceauth.Service[Status]

func New() *Service {
	return deviceauth.New(deviceauth.Config[Status]{
		Command:         "kimi",
		Args:            []string{"login"},
		Env:             kimiEnv,
		NotFound:        ErrKimiNotFound,
		StartErrorLabel: "kimi login",
		ReadyTimeout:    deviceLoginReadyTimeout,
		LoginTimeout:    deviceLoginTimeout,
		LoginTTL:        deviceLoginTTL,
		URLPattern:      deviceURLRE,
		CodePattern:     deviceCodeRE,
		Authenticated:   authenticated,
		BuildStatus: func() deviceauth.StatusBuilder[Status] {
			authenticated := authenticated()
			return func(state deviceauth.State) Status {
				return Status{Authenticated: authenticated, DeviceLogin: state}
			}
		},
		ResolveCompletion: func(err error) deviceauth.Completion {
			switch {
			case authenticated():
				return deviceauth.Completion{Completed: true}
			case err != nil:
				return deviceauth.Completion{Error: fmt.Sprintf("kimi login failed: %s", truncate(err.Error(), 300))}
			default:
				return deviceauth.Completion{Error: "Kimi login ended before authentication completed."}
			}
		},
	})
}

// authenticated reports whether a Kimi Code OAuth credential exists on the
// host (any regular file under ~/.kimi-code/credentials/).
func authenticated() bool {
	entries, err := os.ReadDir(kimiCredsDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			return true
		}
	}
	return false
}

func kimiHomeDir() string {
	if v := os.Getenv("KIMI_CODE_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return filepath.Join(v, ".kimi-code")
	}
	return "/root/.kimi-code"
}

func kimiCredsDir() string {
	return filepath.Join(kimiHomeDir(), "credentials")
}

func kimiEnv(base []string) []string {
	for _, env := range base {
		if strings.HasPrefix(env, "KIMI_CODE_HOME=") {
			return base
		}
	}
	return append(base, "KIMI_CODE_HOME="+kimiHomeDir())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
