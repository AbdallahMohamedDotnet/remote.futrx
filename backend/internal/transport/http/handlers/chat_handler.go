package httphandlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type ChatHandler struct {
	chats    *servicechat.Service
	projects *serviceproject.Service
	auth     *serviceauth.Service
}

func NewChatHandler(
	chats *servicechat.Service,
	projects *serviceproject.Service,
	auth *serviceauth.Service,
) *ChatHandler {
	return &ChatHandler{chats: chats, projects: projects, auth: auth}
}

func (h *ChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/chats", h.HandleCollection)
	mux.HandleFunc("/api/chats/", h.HandleResource)
}

func (h *ChatHandler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	email, isAdmin, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		metas, err := h.chats.List(r.Context())
		if err != nil {
			sendChatError(w, err)
			return
		}
		filtered := h.filterChats(r.Context(), metas, email, isAdmin)
		if filtered == nil {
			filtered = []servicechat.Meta{}
		}
		httptransport.SendJSON(w, http.StatusOK, filtered)

	case http.MethodPost:
		var in servicechat.CreateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil && err != io.EOF {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if in.ProjectID != "" {
			ok, err := h.canAccessProject(r.Context(), serviceproject.ID(in.ProjectID), email, isAdmin)
			if err != nil {
				sendChatError(w, err)
				return
			}
			if !ok {
				httptransport.SendErr(w, http.StatusForbidden, "not a member of this project")
				return
			}
		}
		meta, err := h.chats.Create(r.Context(), in)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusCreated, meta)

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ChatHandler) HandleResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.SplitN(rest, "/", 2)
	id := servicechat.ID(parts[0])

	email, isAdmin, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Resolve the chat once so we can check project membership before any
	// action (read or write). 404 if the chat doesn't exist; 403 if it
	// belongs to a project the caller can't see.
	meta, err := h.chats.Get(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	if meta.ProjectID != "" {
		ok, err := h.canAccessProject(r.Context(), serviceproject.ID(meta.ProjectID), email, isAdmin)
		if err != nil {
			sendChatError(w, err)
			return
		}
		if !ok {
			httptransport.SendErr(w, http.StatusForbidden, "not a member of this chat's project")
			return
		}
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "events":
			h.handleEvents(w, r, id)
		case "rewind":
			h.handleRewind(w, r, id)
		case "read":
			h.handleMarkRead(w, r, id)
		default:
			httptransport.SendErr(w, http.StatusNotFound, "not found")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		httptransport.SendJSON(w, http.StatusOK, meta)

	case http.MethodPatch:
		var in servicechat.UpdateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		updated, err := h.chats.Update(r.Context(), id, in)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := h.chats.Delete(r.Context(), id); err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ChatHandler) handleEvents(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	page, err := h.chats.EventPage(r.Context(), id, servicechat.EventPageQuery{
		Limit:     intQuery(r, "limit", 200),
		BeforeSeq: int64Query(r, "before", 0),
	})
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, page)
}

func (h *ChatHandler) handleRewind(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		BeforeT int64 `json:"beforeT"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := h.chats.Rewind(r.Context(), id, body.BeforeT); err != nil {
		sendChatError(w, err)
		return
	}
	page, err := h.chats.EventPage(r.Context(), id, servicechat.EventPageQuery{Limit: 200})
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, page)
}

func (h *ChatHandler) handleMarkRead(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	meta, err := h.chats.MarkRead(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, meta)
}

// canAccessProject returns true if the caller is an admin OR a member of
// the project. Admins always pass. Members are looked up in the access list.
func (h *ChatHandler) canAccessProject(ctx context.Context, id serviceproject.ID, email string, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if h.projects == nil || email == "" {
		return false, nil
	}
	return h.projects.HasAccess(ctx, id, email)
}

// filterChats removes chats that belong to projects the caller can't see.
// Chats with no projectId are always visible (they predate per-project
// access and don't have a container behind them).
func (h *ChatHandler) filterChats(ctx context.Context, metas []servicechat.Meta, email string, isAdmin bool) []servicechat.Meta {
	if isAdmin || h.projects == nil {
		return metas
	}
	out := make([]servicechat.Meta, 0, len(metas))
	for _, m := range metas {
		if m.ProjectID == "" {
			out = append(out, m)
			continue
		}
		ok, err := h.projects.HasAccess(ctx, serviceproject.ID(m.ProjectID), email)
		if err == nil && ok {
			out = append(out, m)
		}
	}
	return out
}

func (h *ChatHandler) caller(r *http.Request) (string, bool, error) {
	if h.auth == nil {
		return "", true, nil
	}
	return callerStateFromRequest(r.Context(), r, h.auth)
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Query(r *http.Request, key string, fallback int64) int64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func sendChatError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, servicechat.ErrInvalidID),
		errors.Is(err, servicechat.ErrInvalidTmuxSession),
		errors.Is(err, servicechat.ErrInvalidRewindTimestamp):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, servicechat.ErrNotFound):
		httptransport.SendErr(w, http.StatusNotFound, "chat not found")
	case errors.Is(err, servicechat.ErrChatRunning):
		httptransport.SendErr(w, http.StatusConflict, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
