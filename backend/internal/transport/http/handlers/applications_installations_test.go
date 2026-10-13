package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileapplications"
)

func TestProjectInstallationReturnsAcceptedAndKeepsScopeBoundaries(t *testing.T) {
	app := svc.Application{ID: "test-ui", Name: "Test", Scopes: []svc.Scope{svc.ScopeProject, svc.ScopeGlobal}}
	store, err := fileapplications.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := svc.New(&webTestRegistry{application: app}, store, nil, nil, nil)
	handler := NewApplicationsHandler(service, nil, nil)
	post := httptest.NewRequest(http.MethodPost, "/api/projects/p1/applications/installations", strings.NewReader(`{"applicationId":"test-ui"}`))
	accepted := httptest.NewRecorder()
	handler.HandleProject(accepted, post, "p1", []string{"p1", "applications", "installations"})
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("install status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	var started svc.InstallRun
	if err := json.Unmarshal(accepted.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.State != "running" || started.Scope != svc.ScopeProject || started.ProjectID != "p1" {
		t.Fatalf("accepted=%+v", started)
	}
	for _, project := range []string{"p1", "p2"} {
		request := httptest.NewRequest(http.MethodGet, "/api/projects/"+project+"/applications/installations/test-ui", nil)
		response := httptest.NewRecorder()
		handler.HandleProject(response, request, project, []string{project, "applications", "installations/test-ui"})
		if response.Code != http.StatusOK {
			t.Fatalf("status=%d", response.Code)
		}
		if project == "p2" && strings.TrimSpace(response.Body.String()) != "null" {
			t.Fatalf("cross-project log exposed: %s", response.Body.String())
		}
		if project == "p1" && !strings.Contains(response.Body.String(), started.ID) {
			t.Fatal("owner cannot read installation")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for ctx.Err() == nil {
		run, _, _ := service.Installation(ctx, svc.ScopeProject, "p1", app.ID)
		if run.State == "succeeded" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("UI-only installation did not complete")
}

func TestGlobalInstallationEndpointsRequireAuthentication(t *testing.T) {
	store, err := fileapplications.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := svc.New(&webTestRegistry{}, store, nil, nil, nil)
	handler := NewApplicationsHandler(service, nil, nil)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		response := httptest.NewRecorder()
		handler.handleResource(response, httptest.NewRequest(method, "/api/applications/installations", nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s=%d", method, response.Code)
		}
	}
}
