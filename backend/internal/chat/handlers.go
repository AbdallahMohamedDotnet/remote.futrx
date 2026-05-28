package chat

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/projects"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/tmux"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/upload"
)

type TmuxClient interface {
	Cwd(session string) (string, error)
}

// ProjectResolver is the subset of projects.Store this package needs.
// Letting it be nil keeps chat usable on hosts without LXD.
type ProjectResolver interface {
	Get(id string) (projects.ProjectMeta, error)
}

type Handler struct {
	Store    *ChatStore
	Tmux     TmuxClient
	Projects ProjectResolver // nil-safe
}

func NewHandler(store *ChatStore, tmuxClient TmuxClient, projectResolver ProjectResolver) *Handler {
	return &Handler{Store: store, Tmux: tmuxClient, Projects: projectResolver}
}

func (h *Handler) HandleChatsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		metas, err := h.Store.List()
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, metas)
	case http.MethodPost:
		var body struct {
			Title       string `json:"title,omitempty"`
			TmuxSession string `json:"tmuxSession,omitempty"`
			Cwd         string `json:"cwd,omitempty"`
			Model       string `json:"model,omitempty"`
			ProjectID   string `json:"projectId,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil && err != io.EOF {
			httpserver.SendErr(w, 400, "invalid json")
			return
		}
		// Resolve cwd:
		//   - explicit body.Cwd wins
		//   - else if a project is given, the project's HOST workspace path
		//     (uploads write there; claude.Runner translates to /workspace
		//     inside the container when actually spawning claude)
		//   - else from tmux session, if any
		cwd := body.Cwd
		if cwd == "" && body.ProjectID != "" && h.Projects != nil {
			if p, err := h.Projects.Get(body.ProjectID); err == nil {
				cwd = p.Cwd
			}
		}
		if cwd == "" && body.TmuxSession != "" {
			if !tmux.ValidName(body.TmuxSession) {
				httpserver.SendErr(w, 400, "invalid tmuxSession")
				return
			}
			c, err := h.Tmux.Cwd(body.TmuxSession)
			if err == nil {
				cwd = c
			}
		}
		meta, err := h.Store.Create(ChatMeta{
			Title:       body.Title,
			TmuxSession: body.TmuxSession,
			Cwd:         cwd,
			Model:       body.Model,
			ProjectID:   body.ProjectID,
		})
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 201, meta)
	default:
		httpserver.SendErr(w, 405, "method not allowed")
	}
}

func (h *Handler) HandleChatResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	if !validChatID(id) {
		httpserver.SendErr(w, 400, "invalid chat id")
		return
	}

	// /api/chats/{id}/events
	if len(parts) == 2 && parts[1] == "events" {
		if r.Method != http.MethodGet {
			httpserver.SendErr(w, 405, "method not allowed")
			return
		}
		events, err := h.Store.ReadEvents(id)
		if err != nil {
			httpserver.SendErr(w, 500, err.Error())
			return
		}
		httpserver.SendJSON(w, 200, events)
		return
	}

	// /api/chats/{id}/upload — multipart upload into chat's cwd
	if len(parts) == 2 && parts[1] == "upload" {
		if r.Method != http.MethodPost {
			httpserver.SendErr(w, 405, "method not allowed")
			return
		}
		meta, err := h.Store.GetMeta(id)
		if err != nil {
			httpserver.SendErr(w, 404, "chat not found")
			return
		}
		cwd := meta.Cwd
		if meta.TmuxSession != "" {
			if c, e := h.Tmux.Cwd(meta.TmuxSession); e == nil && c != "" {
				cwd = c
			}
		}
		if cwd == "" {
			cwd = os.Getenv("HOME")
			if cwd == "" {
				cwd = "/root"
			}
		}
		upload.HandleMultipart(cwd, w, r)
		return
	}

	// /api/chats/{id}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			meta, err := h.Store.GetMeta(id)
			if err != nil {
				if os.IsNotExist(err) {
					httpserver.SendErr(w, 404, "not found")
				} else {
					httpserver.SendErr(w, 500, err.Error())
				}
				return
			}
			httpserver.SendJSON(w, 200, meta)
		case http.MethodPatch:
			var body struct {
				Title *string `json:"title,omitempty"`
				Cwd   *string `json:"cwd,omitempty"`
				Model *string `json:"model,omitempty"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
				httpserver.SendErr(w, 400, "invalid json")
				return
			}
			meta, err := h.Store.UpdateMeta(id, func(m *ChatMeta) {
				if body.Title != nil {
					m.Title = strings.TrimSpace(*body.Title)
				}
				if body.Cwd != nil {
					m.Cwd = *body.Cwd
				}
				if body.Model != nil {
					m.Model = *body.Model
				}
			})
			if err != nil {
				httpserver.SendErr(w, 500, err.Error())
				return
			}
			httpserver.SendJSON(w, 200, meta)
		case http.MethodDelete:
			if err := h.Store.Delete(id); err != nil {
				httpserver.SendErr(w, 500, err.Error())
				return
			}
			httpserver.SendJSON(w, 200, map[string]bool{"ok": true})
		default:
			httpserver.SendErr(w, 405, "method not allowed")
		}
		return
	}

	httpserver.SendErr(w, 404, "not found")
}
