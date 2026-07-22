package httphandlers

import (
	"net/http"
	"net/url"
	"strings"
)

// returnToCookieName is the short-lived cookie carrying the post-login redirect
// target through the OAuth round-trip.
const returnToCookieName = "return_to"

func baseHost(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func reconstructOriginalURL(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	uri := r.Header.Get("X-Forwarded-Uri")
	if host == "" || uri == "" {
		return ""
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	return proto + "://" + host + uri
}

// isSafeReturnTo permits the configured origin and its subdomains only.
func isSafeReturnTo(rawURL, base string) bool {
	if rawURL == "" || len(rawURL) > 2048 {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false
	}
	bu, err := url.Parse(base)
	if err != nil || bu.Host == "" {
		return false
	}
	return u.Host == bu.Host || strings.HasSuffix(u.Host, "."+bu.Host)
}
