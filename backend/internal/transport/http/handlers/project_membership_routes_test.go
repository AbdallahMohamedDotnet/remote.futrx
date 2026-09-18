package httphandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileauth"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileproject"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileprojectaccess"
	httpmiddleware "github.com/futrx-com/remote.futrx.com/internal/transport/http/middleware"
)

// membershipUserDirectory is an in-memory user directory: the map value is
// whether the account is an administrator.
type membershipUserDirectory map[string]bool

func (d membershipUserDirectory) IsAdmin(_ context.Context, email string) (bool, error) {
	return d[email], nil
}

func (d membershipUserDirectory) IsRegistered(_ context.Context, email string) (bool, error) {
	_, ok := d[email]
	return ok, nil
}

func (membershipUserDirectory) AddBootstrapAdmin(context.Context, string) error { return nil }

func (membershipUserDirectory) FirstAdmin(context.Context) (*serviceauth.UserDirectoryEntry, error) {
	return nil, nil
}

type membershipRoutesFixture struct {
	handler   http.Handler
	auth      *serviceauth.Service
	projectID string
}

func newMembershipRoutesFixture(t *testing.T) membershipRoutesFixture {
	t.Helper()
	repo, err := fileproject.NewWithWorkspaceRoot(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	access, err := fileprojectaccess.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := serviceproject.New(repo, serviceproject.ContainerDependencies{}, nil, access)
	project, err := projects.Create(
		context.Background(), serviceproject.CreateInput{Name: "Members Only"}, "member@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := serviceauth.New(
		context.Background(),
		fileauth.New(t.TempDir()),
		membershipUserDirectory{
			"admin@example.com":    true,
			"member@example.com":   false,
			"outsider@example.com": false,
		},
		func(string, string, string) serviceauth.OAuthProvider { return verifyOAuthProvider{} },
		"https://"+verifyBaseHost,
		[]byte("membership-routes-test-key"),
		twoFactorStoreForTest(t),
		sessionRegistryStoreForTest(t),
		testAuthOptions(),
	)
	if err != nil {
		t.Fatalf("New auth service: %v", err)
	}
	mux := http.NewServeMux()
	NewProjectHandler(projects, nil, auth, sharesPublicHostname, nil).RegisterRoutes(mux)
	return membershipRoutesFixture{
		handler:   httpmiddleware.NewAuth(auth).Wrap(mux),
		auth:      auth,
		projectID: string(project.ID),
	}
}

func (f membershipRoutesFixture) do(t *testing.T, email, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if email != "" {
		session, err := f.auth.IssueSession(
			context.Background(), serviceauth.User{Email: email}, serviceauth.SignInMethodPassword, "", "",
		)
		if err != nil {
			t.Fatalf("issue session: %v", err)
		}
		request.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: session})
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	return response
}

// The project service does not authorize its callers today: these routes are
// protected only by the handler's admin-or-member check. This pins that
// visible behavior before authorization moves into the service.
func TestProjectLifecycleAndAccessRoutesRequireMembershipOrAdministrator(t *testing.T) {
	fixture := newMembershipRoutesFixture(t)
	base := "/api/projects/" + fixture.projectID

	routes := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"start", http.MethodPost, base + "/start", ""},
		{"stop", http.MethodPost, base + "/stop", ""},
		{"restart", http.MethodPost, base + "/restart", ""},
		{"repair network", http.MethodPost, base + "/repair-network", ""},
		{"list access", http.MethodGet, base + "/access", ""},
	}
	callers := []struct {
		name       string
		email      string
		wantStatus int
	}{
		{"member", "member@example.com", http.StatusOK},
		{"administrator", "admin@example.com", http.StatusOK},
		{"registered non-member", "outsider@example.com", http.StatusForbidden},
	}
	for _, route := range routes {
		for _, caller := range callers {
			t.Run(route.name+"/"+caller.name, func(t *testing.T) {
				response := fixture.do(t, caller.email, route.method, route.path, route.body)
				if response.Code != caller.wantStatus {
					t.Fatalf("status = %d, want %d, body = %s", response.Code, caller.wantStatus, response.Body.String())
				}
			})
		}
	}
}

func TestProjectAccessMutationRoutesRequireMembershipOrAdministrator(t *testing.T) {
	fixture := newMembershipRoutesFixture(t)
	base := "/api/projects/" + fixture.projectID + "/access"

	forbidden := fixture.do(t, "outsider@example.com", http.MethodPost, base, `{"email":"outsider@example.com"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-member add status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	added := fixture.do(t, "member@example.com", http.MethodPost, base, `{"email":"outsider@example.com"}`)
	if added.Code != http.StatusOK {
		t.Fatalf("member add status = %d, body = %s", added.Code, added.Body.String())
	}
	removed := fixture.do(t, "admin@example.com", http.MethodDelete, base+"/outsider@example.com", "")
	if removed.Code != http.StatusOK {
		t.Fatalf("administrator remove status = %d, body = %s", removed.Code, removed.Body.String())
	}
}

func TestProjectRoutesRejectAnonymousCallers(t *testing.T) {
	fixture := newMembershipRoutesFixture(t)

	response := fixture.do(t, "", http.MethodPost, "/api/projects/"+fixture.projectID+"/start", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
