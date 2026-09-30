package httphandlers

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
)

var webRoutePart = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// serveWeb routes an installed project's declared web service. The project
// list is scoped to this caller, and the port comes only from a validated
// catalog manifest; neither the URL nor client headers choose the upstream.
func (h *ApplicationsHandler) serveWeb(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/apps/"), "/", 3)
	if len(parts) < 2 || !webRoutePart.MatchString(parts[0]) || !webRoutePart.MatchString(parts[1]) {
		http.NotFound(w, r)
		return
	}
	if !h.requireRegistered(w, r) || h.apps == nil {
		return
	}
	if len(parts) == 2 || parts[2] == "" && !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, r.URL.Path+"/"+querySuffix(r.URL.RawQuery), http.StatusPermanentRedirect)
		return
	}
	if h.projects == nil {
		http.NotFound(w, r)
		return
	}
	email, err := callerEmailFromRequest(r, h.auth)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	isAdmin, _ := h.auth.IsAdmin(r.Context(), email)
	projects, err := h.projects.ListVisible(r.Context(), email, isAdmin)
	if err != nil {
		http.Error(w, "project access unavailable", http.StatusInternalServerError)
		return
	}
	projectID := ""
	for _, project := range projects {
		if project.Slug == parts[0] {
			projectID = string(project.ID)
			break
		}
	}
	if projectID == "" {
		http.NotFound(w, r)
		return
	}
	port, available, err := h.apps.WebPort(r.Context(), projectID, parts[1])
	if err != nil {
		http.Error(w, "application unavailable", http.StatusInternalServerError)
		return
	}
	if !available {
		http.NotFound(w, r)
		return
	}
	upstream := &url.URL{Scheme: "http", Host: net.JoinHostPort(parts[0]+".lxd", fmt.Sprint(port))}
	prefix := "/apps/" + parts[0] + "/" + parts[1]
	newWebProxy(upstream, prefix).ServeHTTP(w, r)
}

func newWebProxy(upstream *url.URL, prefix string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, prefix)
			pr.Out.URL.RawPath = ""
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
			pr.Out.Header.Del("Cookie")
			pr.SetXForwarded()
		},
		Transport: &http.Transport{DisableKeepAlives: true, Proxy: nil},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			// An application shares Remote's origin. Never let its service worker
			// claim the main UI or another project's application routes.
			if response.Header.Get("Service-Worker-Allowed") != "" {
				response.Header.Set("Service-Worker-Allowed", prefix+"/")
			}
			if location := response.Header.Get("Location"); strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
				response.Header.Set("Location", prefix+location)
			}
			return nil
		},
	}
}

func querySuffix(raw string) string {
	if raw == "" {
		return ""
	}
	return "?" + raw
}
