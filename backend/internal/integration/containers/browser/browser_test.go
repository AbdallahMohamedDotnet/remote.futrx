package browser

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func TestParseAgentBrowserStatus(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want serviceproject.AgentBrowserInfo
	}{
		{
			name: "empty output is stopped",
			want: serviceproject.AgentBrowserInfo{
				Status: serviceproject.AgentBrowserStatusStopped,
				Core:   "off",
				View:   "off",
			},
		},
		{
			name: "core and view ready",
			out:  "core=ready view=ready clients=3 uptime_sec=42",
			want: serviceproject.AgentBrowserInfo{
				Status:      serviceproject.AgentBrowserStatusReady,
				Core:        "ready",
				View:        "ready",
				ViewerCount: 3,
				UptimeSec:   42,
			},
		},
		{
			name: "core ready without view",
			out:  "view=off core=ready",
			want: serviceproject.AgentBrowserInfo{
				Status: serviceproject.AgentBrowserStatusCoreReady,
				Core:   "ready",
				View:   "off",
			},
		},
		{
			name: "view alone remains stopped and invalid counts are ignored",
			out:  "core=off view=ready clients=many uptime_sec=unknown ignored=value",
			want: serviceproject.AgentBrowserInfo{
				Status: serviceproject.AgentBrowserStatusStopped,
				Core:   "off",
				View:   "ready",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAgentBrowserStatus(tt.out); got != tt.want {
				t.Fatalf("parseAgentBrowserStatus(%q) = %#v, want %#v", tt.out, got, tt.want)
			}
		})
	}
}

func TestEnsureProvisionsBeforeStartingWithExpectedVerb(t *testing.T) {
	tests := []struct {
		name   string
		ensure func(*Service, context.Context, string) error
		verb   string
	}{
		{name: "full stack", ensure: (*Service).Ensure, verb: "start"},
		{name: "core", ensure: (*Service).EnsureCore, verb: "start-core"},
		{name: "view", ensure: (*Service).EnsureView, verb: "start-view"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &browserRecordingRunner{}
			service := NewService(runner, nil, assets.NewPublisher(runner))

			if err := tt.ensure(service, context.Background(), "c1"); err != nil {
				t.Fatalf("ensure browser: %v", err)
			}

			want := []string{
				"exec c1 -- sh -c command -v Xvfb >/dev/null 2>&1 && ls /root/.cache/ms-playwright/chromium-*/chrome-linux64/chrome >/dev/null 2>&1",
				"exec c1 -- install -d -m 755 " + containerGUIDir,
				"exec c1 -- cat " + containerGUIScriptHash,
				"exec c1 -- cat " + containerHumanInputHash,
				"exec c1 -- sh " + containerGUIScript + " " + tt.verb,
			}
			if got := strings.Join(runner.calls, "\n"); got != strings.Join(want, "\n") {
				t.Fatalf("command order:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
			}
		})
	}
}

type browserRecordingRunner struct {
	calls []string
}

func (r *browserRecordingRunner) Available() bool { return true }

func (r *browserRecordingRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, strings.Join(args, " "))
	switch args[len(args)-1] {
	case containerGUIScriptHash:
		return assets.Hash(guiUpScript), nil
	case containerHumanInputHash:
		return assets.Hash(humanInputScript), nil
	default:
		return "", nil
	}
}

func (r *browserRecordingRunner) RunStdin(context.Context, io.Reader, ...string) (string, error) {
	return "", nil
}
