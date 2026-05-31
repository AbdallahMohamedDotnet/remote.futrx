package httphandlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type ChatHandler struct {
	chats *servicechat.Service
}

func NewChatHandler(chats *servicechat.Service) *ChatHandler {
	return &ChatHandler{chats: chats}
}

func (h *ChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/chats", h.HandleCollection)
	mux.HandleFunc("/api/chats/", h.HandleResource)
}

func (h *ChatHandler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		metas, err := h.chats.List(r.Context())
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, metas)

	case http.MethodPost:
		var in servicechat.CreateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil && err != io.EOF {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
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
		meta, err := h.chats.Get(r.Context(), id)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, meta)

	case http.MethodPatch:
		var in servicechat.UpdateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		meta, err := h.chats.Update(r.Context(), id, in)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, meta)

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
