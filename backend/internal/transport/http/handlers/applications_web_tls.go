package httphandlers

import (
	"net/http"

	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

// serveApplicationTLSAsk admits certificates only for a currently routable app.
// The caller validates the method, domain parameter and application namespace.
func (h *ProjectHandler) serveApplicationTLSAsk(w http.ResponseWriter, r *http.Request, domain string) {
	if h.apps == nil || h.apps.apps == nil {
		http.NotFound(w, r)
		return
	}
	label, slug, named := httptransport.ApplicationProject(domain, h.publicHostname)
	if named {
		project, err := h.projects.GetBySlug(r.Context(), slug)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, available, err := h.apps.apps.ProjectWebTargetBySubdomain(r.Context(), string(project.ID), label)
		if err != nil || !available {
			http.NotFound(w, r)
			return
		}
	} else {
		id, valid := httptransport.ApplicationInstanceID(domain, h.publicHostname)
		if !valid {
			http.NotFound(w, r)
			return
		}
		target, available, err := h.apps.apps.WebTarget(r.Context(), id)
		if err != nil || !available || !httptransport.MatchesApplicationHost(domain, target.InstanceID, target.Subdomain, h.publicHostname) {
			http.NotFound(w, r)
			return
		}
		if _, err := h.projects.Get(r.Context(), serviceproject.ID(target.ProjectID)); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
