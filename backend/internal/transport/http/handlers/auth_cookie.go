package httphandlers

import (
	"net/http"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
)

// pendingTwoFactorCookieName carries the short-lived pending-login token
// between a completed first factor (password/Google) and the 2FA challenge
// endpoint. 5 minutes matches the pending-login token's own TTL.
const (
	pendingTwoFactorCookieName   = "remote_2fa_pending"
	pendingTwoFactorCookieMaxAge = 5 * 60
)

func setSessionCookie(w http.ResponseWriter, auth *serviceauth.Service, cookieValue string) {
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Value: cookieValue,
		Path: "/", Domain: auth.CookieDomain(),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(serviceauth.SessionDuration().Seconds()),
	})
}

func setPendingCookie(w http.ResponseWriter, auth *serviceauth.Service, pendingToken string) {
	http.SetCookie(w, &http.Cookie{
		Name: pendingTwoFactorCookieName, Value: pendingToken,
		Path: "/", Domain: auth.CookieDomain(),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: pendingTwoFactorCookieMaxAge,
	})
}

func clearPendingCookie(w http.ResponseWriter, auth *serviceauth.Service) {
	http.SetCookie(w, &http.Cookie{
		Name: pendingTwoFactorCookieName, Path: "/", Domain: auth.CookieDomain(), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

func pendingCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(pendingTwoFactorCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
