package httphandlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	serviceusersettings "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/usersettings"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

// ProjectLister enumerates projects so we know which containers to push
// secret updates into.
type ProjectLister interface {
	List(ctx context.Context) ([]serviceproject.Meta, error)
}

// ContainerEnvApplier applies env-var changes to one container.
type ContainerEnvApplier interface {
	ApplyContainerEnvDiff(ctx context.Context, container string, set map[string]string, unset []string) error
}

type UserSettingsHandler struct {
	settings   *serviceusersettings.Service
	auth       *serviceauth.Service
	projects   ProjectLister
	containers ContainerEnvApplier
}

func NewUserSettingsHandler(
	settings *serviceusersettings.Service,
	auth *serviceauth.Service,
	projects ProjectLister,
	containers ContainerEnvApplier,
) *UserSettingsHandler {
	return &UserSettingsHandler{
		settings:   settings,
		auth:       auth,
		projects:   projects,
		containers: containers,
	}
}

func (h *UserSettingsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/me/settings", h.HandleSettings)
}

// settingsView is the HTTP-facing shape of Settings. Crucially, Secrets
// values are replaced with the constant "set" so the API never returns the
// plaintext to a browser — a stolen session cookie cannot exfiltrate tokens.
type settingsView struct {
	Appearance serviceusersettings.Appearance `json:"appearance"`
	Secrets    map[string]string              `json:"secrets,omitempty"`
	UpdatedAt  int64                          `json:"updatedAt,omitempty"`
}

type patchResult struct {
	settingsView
	// Propagated is the number of containers the new secrets were pushed to.
	// Failures (if any) carry the per-container error so the UI can surface
	// them; the rest of the fleet still gets updated.
	Propagated int               `json:"propagated"`
	Failures   []containerFailure `json:"failures,omitempty"`
}

type containerFailure struct {
	Container string `json:"container"`
	Error     string `json:"error"`
}

func maskedView(s serviceusersettings.Settings) settingsView {
	v := settingsView{
		Appearance: s.Appearance,
		UpdatedAt:  s.UpdatedAt,
	}
	if len(s.Secrets) > 0 {
		v.Secrets = make(map[string]string, len(s.Secrets))
		for k := range s.Secrets {
			v.Secrets[k] = "set"
		}
	}
	return v
}

func (h *UserSettingsHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		httptransport.SendErr(w, http.StatusServiceUnavailable, "user settings unavailable")
		return
	}

	key, err := h.key(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := h.settings.Get(r.Context(), key)
		if err != nil {
			sendUserSettingsError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, maskedView(settings))

	case http.MethodPatch:
		var input serviceusersettings.UpdateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&input); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		settings, diff, err := h.settings.Update(r.Context(), key, input)
		if err != nil {
			sendUserSettingsError(w, err)
			return
		}

		result := patchResult{settingsView: maskedView(settings)}
		if !diff.Empty() && h.containers != nil && h.projects != nil {
			result.Propagated, result.Failures = h.propagateSecrets(r.Context(), diff)
		}
		httptransport.SendJSON(w, http.StatusOK, result)

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// propagateSecrets pushes the diff into every project's container. Errors
// are collected per-container so a single bad container doesn't break the
// rest of the fleet, and the user sees exactly which ones failed.
func (h *UserSettingsHandler) propagateSecrets(
	ctx context.Context,
	diff serviceusersettings.SettingsDiff,
) (int, []containerFailure) {
	projects, err := h.projects.List(ctx)
	if err != nil {
		log.Printf("user-settings: list projects for secrets propagation: %v", err)
		return 0, []containerFailure{{Container: "<list-projects>", Error: err.Error()}}
	}

	ok := 0
	var failures []containerFailure
	for _, p := range projects {
		if p.ContainerName == "" {
			continue
		}
		if err := h.containers.ApplyContainerEnvDiff(
			ctx, p.ContainerName, diff.SecretsSet, diff.SecretsUnset,
		); err != nil {
			log.Printf("user-settings: propagate to %s: %v", p.ContainerName, err)
			failures = append(failures, containerFailure{
				Container: p.ContainerName,
				Error:     err.Error(),
			})
			continue
		}
		ok++
	}
	return ok, failures
}

func (h *UserSettingsHandler) key(r *http.Request) (serviceusersettings.Key, error) {
	if h.auth == nil {
		return serviceusersettings.LocalAdminKey, nil
	}

	cookie, err := r.Cookie(serviceauth.SessionCookieName)
	if err != nil {
		return "", err
	}
	session, err := h.auth.CurrentSession(cookie.Value)
	if err != nil {
		return "", err
	}
	return serviceusersettings.KeyFromSession(session.Email, session.Sub)
}

func sendUserSettingsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, serviceusersettings.ErrInvalidIdentity):
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, serviceusersettings.ErrInvalidTheme),
		errors.Is(err, serviceusersettings.ErrInvalidEnvKey):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
