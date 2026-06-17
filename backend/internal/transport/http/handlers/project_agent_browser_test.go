package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/fileproject"
)

func TestProjectAgentBrowserRoutes(t *testing.T) {
	handler, containers, project := newAgentBrowserProjectHandler(t)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+string(project.ID)+"/agent-browser", nil)
	statusReq.Host = "remote.futrx.dev"
	statusRec := httptest.NewRecorder()
	handler.HandleResource(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var status agentBrowserResponse
	if err := json.NewDecoder(statusRec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Status != serviceproject.AgentBrowserStatusStopped || status.URL != "" || status.Slug != project.Slug || status.Port != 6080 {
		t.Fatalf("GET response = %#v", status)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+string(project.ID)+"/agent-browser/start", strings.NewReader("{}"))
	startReq.Host = "remote.futrx.dev"
	startReq.Header.Set("X-Forwarded-Proto", "https")
	startRec := httptest.NewRecorder()
	handler.HandleResource(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("POST start = %d body=%s", startRec.Code, startRec.Body.String())
	}
	if !containers.agentBrowserStarted {
		t.Fatal("expected container Agent Browser start")
	}
	var started agentBrowserResponse
	if err := json.NewDecoder(startRec.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	wantURL := "https://" + project.Slug + "--6080.dev.remote.futrx.dev/vnc.html?autoconnect=1&resize=scale&reconnect=1"
	if started.Status != serviceproject.AgentBrowserStatusReady || started.URL != wantURL || started.Slug != project.Slug || started.Port != 6080 {
		t.Fatalf("POST response = %#v, want url %q", started, wantURL)
	}

	stopReq := httptest.NewRequest(http.MethodDelete, "/api/projects/"+string(project.ID)+"/agent-browser", nil)
	stopRec := httptest.NewRecorder()
	handler.HandleResource(stopRec, stopReq)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("DELETE stop = %d body=%s", stopRec.Code, stopRec.Body.String())
	}
	if !containers.agentBrowserStopped {
		t.Fatal("expected container Agent Browser stop")
	}
	var stopped map[string]serviceproject.AgentBrowserStatus
	if err := json.NewDecoder(stopRec.Body).Decode(&stopped); err != nil {
		t.Fatal(err)
	}
	if stopped["status"] != serviceproject.AgentBrowserStatusStopped {
		t.Fatalf("DELETE response = %#v", stopped)
	}
}

func TestProjectAgentBrowserRouteMethods(t *testing.T) {
	handler, _, project := newAgentBrowserProjectHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+string(project.ID)+"/agent-browser", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	handler.HandleResource(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /agent-browser status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+string(project.ID)+"/agent-browser/start", nil)
	rec = httptest.NewRecorder()
	handler.HandleResource(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /agent-browser/start status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func newAgentBrowserProjectHandler(t *testing.T) (*ProjectHandler, *fakeProjectContainers, serviceproject.Meta) {
	t.Helper()
	repo, err := fileproject.NewWithWorkspaceRoot(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	containers := &fakeProjectContainers{}
	projects := serviceproject.New(repo, containers, nil, nil)
	project, err := projects.Create(context.Background(), serviceproject.CreateInput{Name: "Browser Project"}, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return NewProjectHandler(projects, nil, nil), containers, project
}

type fakeProjectContainers struct {
	agentBrowserRunning bool
	agentBrowserStarted bool
	agentBrowserStopped bool
}

func (f *fakeProjectContainers) Available() bool { return true }

func (f *fakeProjectContainers) Launch(context.Context, serviceproject.Meta) error { return nil }

func (f *fakeProjectContainers) Start(context.Context, string) error { return nil }

func (f *fakeProjectContainers) Stop(context.Context, string) error { return nil }

func (f *fakeProjectContainers) Delete(context.Context, string) error { return nil }

func (f *fakeProjectContainers) State(context.Context, string) (serviceproject.ContainerState, error) {
	return serviceproject.ContainerStateRunning, nil
}

func (f *fakeProjectContainers) Inspect(context.Context, string) (serviceproject.ContainerInspect, error) {
	return serviceproject.ContainerInspect{}, nil
}

func (f *fakeProjectContainers) ListListeners(context.Context, string) ([]serviceproject.ContainerApp, error) {
	return nil, nil
}

func (f *fakeProjectContainers) ApplyContainerEnvDiff(context.Context, string, map[string]string, []string) error {
	return nil
}

func (f *fakeProjectContainers) EnsureAgentBrowser(context.Context, string) error {
	f.agentBrowserRunning = true
	f.agentBrowserStarted = true
	return nil
}

func (f *fakeProjectContainers) StopAgentBrowser(context.Context, string) error {
	f.agentBrowserRunning = false
	f.agentBrowserStopped = true
	return nil
}

func (f *fakeProjectContainers) AgentBrowserRunning(context.Context, string) (bool, error) {
	return f.agentBrowserRunning, nil
}

func (f *fakeProjectContainers) AgentBrowserPort() int { return 6080 }
