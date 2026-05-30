package httphandlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/claudelogin"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type ClaudeAuthHandler struct {
	login *claudelogin.Manager
}

func NewClaudeAuthHandler(login *claudelogin.Manager) *ClaudeAuthHandler {
	return &ClaudeAuthHandler{login: login}
}

func (h *ClaudeAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/claude/auth-status", h.HandleStatus)
	mux.HandleFunc("/api/claude/login/start", h.HandleStart)
	mux.HandleFunc("/api/claude/login/code", h.HandleCode)
	mux.HandleFunc("/api/claude/login/cancel", h.HandleCancel)
}

func (h *ClaudeAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, map[string]any{
		"authenticated": h.login.Authenticated(),
	})
}

func (h *ClaudeAuthHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := h.login.Start(r.Context())
	if err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := map[string]any{"url": result.URL}
	if result.Resumed {
		out["resumed"] = true
	}
	httptransport.SendJSON(w, http.StatusOK, out)
}

func (h *ClaudeAuthHandler) HandleCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := readJSONBody(r, &body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.login.SubmitCode(r.Context(), body.Code); err != nil {
		sendClaudeLoginError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *ClaudeAuthHandler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := h.login.Cancel(r.Context()); err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func sendClaudeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claudelogin.ErrCodeRequired),
		errors.Is(err, claudelogin.ErrNoSession):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}

func readJSONBody(r *http.Request, v any) error {
	const max = 1 << 16
	body := http.MaxBytesReader(nil, r.Body, max)
	defer body.Close()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}
