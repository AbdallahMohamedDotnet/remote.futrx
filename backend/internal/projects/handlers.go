package projects

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		metas, err := h.store.List()
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, metas)
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httpserver.SendErr(w, 400, "invalid json")
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			httpserver.SendErr(w, 400, "name is required")
			return
		}
		m, err := h.store.Create(CreateInput{Name: name})
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 201, m)
	default:
		httpserver.SendErr(w, 405, "method not allowed")
	}
}

func (h *Handler) HandleResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	if id == "" {
		httpserver.SendErr(w, 400, "missing id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		m, err := h.store.Get(id)
		if err != nil {
			httpserver.SendErr(w, 404, "not found")
			return
		}
		httpserver.SendJSON(w, 200, m)
	case http.MethodPatch:
		var body struct {
			Name *string `json:"name,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httpserver.SendErr(w, 400, "invalid json")
			return
		}
		m, err := h.store.Update(id, UpdateInput{Name: body.Name})
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, m)
	case http.MethodDelete:
		if err := h.store.Delete(id); err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, map[string]bool{"ok": true})
	default:
		httpserver.SendErr(w, 405, "method not allowed")
	}
}
