package httptransport

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/tmuxcli"
)

type TmuxClient interface {
	List() []tmuxcli.Session
	Create(name string) error
	Kill(name string) error
	Has(name string) bool
	Cwd(session string) (string, error)
	SendText(session, text string, pressEnter bool) error
}

type TmuxHandler struct {
	client TmuxClient
}

func NewTmuxHandler(client TmuxClient) *TmuxHandler {
	return &TmuxHandler{client: client}
}

func (h *TmuxHandler) HandleSessionsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		SendJSON(w, 200, h.client.List())
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
			SendErr(w, 400, "invalid json")
			return
		}
		name := strings.TrimSpace(body.Name)
		if !tmuxcli.ValidName(name) {
			SendErr(w, 400, "invalid name (alphanumeric, _ -, 1-32 chars)")
			return
		}
		if h.client.Has(name) {
			SendErr(w, 409, "session exists")
			return
		}
		if err := h.client.Create(name); err != nil {
			SendErr(w, 500, err.Error())
			return
		}
		SendJSON(w, 201, map[string]string{"name": name})
	default:
		SendErr(w, 405, "method not allowed")
	}
}

func (h *TmuxHandler) HandleSessionResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]
	if !tmuxcli.ValidName(name) {
		SendErr(w, 400, "invalid name")
		return
	}

	// /api/sessions/{name}/upload — multipart upload(s) into the session's cwd.
	if len(parts) == 2 && parts[1] == "upload" {
		if r.Method != http.MethodPost {
			SendErr(w, 405, "method not allowed")
			return
		}
		if !h.client.Has(name) {
			SendErr(w, 404, "session not found")
			return
		}
		cwd, err := h.client.Cwd(name)
		if err != nil || cwd == "" {
			SendErr(w, 500, "could not resolve session cwd")
			return
		}
		HandleMultipart(cwd, w, r)
		return
	}

	// /api/sessions/{name}/send — POST a chat message into the tmux session.
	if len(parts) == 2 && parts[1] == "send" {
		if r.Method != http.MethodPost {
			SendErr(w, 405, "method not allowed")
			return
		}
		if !h.client.Has(name) {
			SendErr(w, 404, "session not found")
			return
		}
		var body struct {
			Text       string `json:"text"`
			PressEnter *bool  `json:"pressEnter,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			SendErr(w, 400, "invalid json")
			return
		}
		pressEnter := true
		if body.PressEnter != nil {
			pressEnter = *body.PressEnter
		}
		if body.Text == "" && !pressEnter {
			SendJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if err := h.client.SendText(name, body.Text, pressEnter); err != nil {
			SendErr(w, 500, err.Error())
			return
		}
		SendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	// /api/sessions/{name} — DELETE.
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			SendErr(w, 405, "method not allowed")
			return
		}
		if !h.client.Has(name) {
			SendErr(w, 404, "not found")
			return
		}
		if err := h.client.Kill(name); err != nil {
			SendErr(w, 500, err.Error())
			return
		}
		SendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	SendErr(w, 404, "not found")
}
