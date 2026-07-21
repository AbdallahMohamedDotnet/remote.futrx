package httpmiddleware

import (
	"net/http"
	"strings"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
)

type Auth struct {
	auth *serviceauth.Service
}

func NewAuth(auth *serviceauth.Service) *Auth {
	return &Auth{auth: auth}
}

func (m *Auth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") {
			next.ServeHTTP(w, r)
			return
		}
		if path == "/auth/me" || strings.HasPrefix(path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		session, err := m.auth.CurrentSession(sessionCookieValue(r))
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		registered, _ := m.auth.IsRegistered(r.Context(), session.Email)
		if !registered {
			http.Error(w, "account not authorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sessionCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(serviceauth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
