package httphandlers

import (
	"net/http"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
)

func setSessionCookie(w http.ResponseWriter, auth *serviceauth.Service, user serviceauth.User) {
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Value: auth.SignSession(user),
		Path: "/", Domain: auth.CookieDomain(),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(serviceauth.SessionDuration().Seconds()),
	})
}
