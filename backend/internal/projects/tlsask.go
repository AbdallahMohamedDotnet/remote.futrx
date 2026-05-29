package projects

// On-demand TLS ask endpoint for the *.dev.remote.futrx.dev wildcard.
//
// Caddy issues a per-host cert lazily the first time a dev URL is hit. To
// avoid having Caddy chase certs for arbitrary names an attacker could
// spam, Caddy's on_demand_tls.ask first queries this endpoint with the
// requested SNI; we return 200 only for hosts that match our pattern AND
// reference a real project.
//
// Pattern: <slug>--<port>.dev.remote.futrx.dev
//   - slug: project slug as stored in the project meta
//   - port: TCP port the container exposes (1024–65535)
//
// This endpoint MUST NOT be reachable from outside — Caddy reaches it via
// loopback (http://localhost:7682/internal/tls-ask). The public Caddy block
// explicitly blocks /internal/* to enforce that.

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// hostPattern accepts only the exact subdomain shape we route. Slug syntax
// matches what slug.go validates at project create time.
var hostPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)--(\d{4,5})\.dev\.remote\.futrx\.dev$`)

// HandleTLSAsk answers Caddy's ask probe before each cert issuance.
func (h *Handler) HandleTLSAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if domain == "" {
		http.Error(w, "missing domain", http.StatusBadRequest)
		return
	}
	m := hostPattern.FindStringSubmatch(domain)
	if m == nil {
		http.Error(w, "host does not match <slug>--<port>.dev.remote.futrx.dev", http.StatusNotFound)
		return
	}
	slug := m[1]
	port, err := strconv.Atoi(m[2])
	if err != nil || port < 1024 || port > 65535 {
		http.Error(w, "port out of range", http.StatusNotFound)
		return
	}
	if _, err := h.store.GetBySlug(slug); err != nil {
		http.Error(w, "no such project", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
