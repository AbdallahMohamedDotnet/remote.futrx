package tmux

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/upload"
)

func (c *Client) HandleSessionsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		httpserver.SendJSON(w, 200, c.List())
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
			httpserver.SendErr(w, 400, "invalid json")
			return
		}
		name := strings.TrimSpace(body.Name)
		if !ValidName(name) {
			httpserver.SendErr(w, 400, "invalid name (alphanumeric, _ -, 1-32 chars)")
			return
		}
		if c.Has(name) {
			httpserver.SendErr(w, 409, "session exists")
			return
		}
		if err := c.Create(name); err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 201, map[string]string{"name": name})
	default:
		httpserver.SendErr(w, 405, "method not allowed")
	}
}

func (c *Client) HandleSessionResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]
	if !ValidName(name) {
		httpserver.SendErr(w, 400, "invalid name")
		return
	}

	// /api/sessions/{name}/upload — multipart upload(s) into the session's cwd.
	if len(parts) == 2 && parts[1] == "upload" {
		if r.Method != http.MethodPost {
			httpserver.SendErr(w, 405, "method not allowed")
			return
		}
		if !c.Has(name) {
			httpserver.SendErr(w, 404, "session not found")
			return
		}
		cwd, err := c.Cwd(name)
		if err != nil || cwd == "" {
			httpserver.SendErr(w, 500, "could not resolve session cwd")
			return
		}
		upload.HandleMultipart(cwd, w, r)
		return
	}

	// /api/sessions/{name}/send — POST a chat message into the tmux session.
	if len(parts) == 2 && parts[1] == "send" {
		if r.Method != http.MethodPost {
			httpserver.SendErr(w, 405, "method not allowed")
			return
		}
		if !c.Has(name) {
			httpserver.SendErr(w, 404, "session not found")
			return
		}
		var body struct {
			Text       string `json:"text"`
			PressEnter *bool  `json:"pressEnter,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			httpserver.SendErr(w, 400, "invalid json")
			return
		}
		pressEnter := true
		if body.PressEnter != nil {
			pressEnter = *body.PressEnter
		}
		if body.Text == "" && !pressEnter {
			httpserver.SendJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if err := c.SendText(name, body.Text, pressEnter); err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	// /api/sessions/{name} — DELETE.
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			httpserver.SendErr(w, 405, "method not allowed")
			return
		}
		if !c.Has(name) {
			httpserver.SendErr(w, 404, "not found")
			return
		}
		if err := c.Kill(name); err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	httpserver.SendErr(w, 404, "not found")
}
