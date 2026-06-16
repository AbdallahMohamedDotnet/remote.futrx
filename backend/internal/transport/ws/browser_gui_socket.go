package wstransport

// BrowserGUISocket is the control channel for the Agent Browser GUI. Unlike
// the terminal socket it does NOT carry pixels — the noVNC view is served by
// the container's websockify front and reaches the browser through the
// existing dev-URL proxy (<slug>--<port>.dev.<host>, behind the platform's
// Google auth). This socket only drives lifecycle: it starts the GUI stack on
// connect, streams "starting" -> "ready"/"error" status, and accepts an
// explicit {"type":"stop"} to tear it down.
//
// It deliberately does NOT stop the GUI when the client disconnects: the same
// browser session is shared with the in-container agent over CDP, so closing
// the view must not kill a browser the agent may still be driving. Idle
// garbage-collection that also accounts for agent activity is a follow-up.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/gorilla/websocket"
)

// browserGUINoVNCPort is the in-container noVNC port the ready message reports
// to the client, which builds the dev-URL from it. Must match
// templates/gui-up.sh and containers.BrowserGUIVNCPort.
const browserGUINoVNCPort = 6080

// BrowserGUIProjects is the subset of the project service this socket needs.
type BrowserGUIProjects interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	StartBrowserGUI(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	StopBrowserGUI(ctx context.Context, id serviceproject.ID) error
}

type BrowserGUISocket struct {
	chats    TerminalChatGetter
	projects BrowserGUIProjects
	access   ProjectAccessChecker
}

func NewBrowserGUISocket(chats TerminalChatGetter, projects BrowserGUIProjects) *BrowserGUISocket {
	return &BrowserGUISocket{chats: chats, projects: projects}
}

func (s *BrowserGUISocket) WithAccessChecker(access ProjectAccessChecker) *BrowserGUISocket {
	s.access = access
	return s
}

func (s *BrowserGUISocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/browser-gui", s.Handle(upgrader))
}

func (s *BrowserGUISocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

type browserGUIServerMsg struct {
	Type    string `json:"type"`
	Slug    string `json:"slug,omitempty"`
	Port    int    `json:"port,omitempty"`
	Message string `json:"message,omitempty"`
}

type browserGUIClientMsg struct {
	Type string `json:"type"`
}

func (s *BrowserGUISocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	chatID := servicechat.ID(strings.TrimSpace(r.URL.Query().Get("chat")))
	if !servicechat.ValidID(chatID) {
		http.Error(w, "invalid chat id", http.StatusBadRequest)
		return
	}
	if s.chats == nil || s.projects == nil {
		http.Error(w, "browser GUI unavailable", http.StatusServiceUnavailable)
		return
	}

	meta, err := s.chats.Get(r.Context(), chatID)
	if err != nil {
		http.Error(w, "chat not found", http.StatusNotFound)
		return
	}
	if meta.ProjectID == "" {
		http.Error(w, "chat has no project container", http.StatusBadRequest)
		return
	}

	if s.access != nil {
		email, isAdmin, err := s.access.CallerAndAdmin(r.Context(), r)
		if err != nil || email == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !isAdmin {
			ok, err := s.access.HasAccess(r.Context(), serviceproject.ID(meta.ProjectID), email)
			if err != nil {
				http.Error(w, "access check failed", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "not a member of this project", http.StatusForbidden)
				return
			}
		}
	}

	projectID := serviceproject.ID(meta.ProjectID)
	if _, err := s.projects.Get(r.Context(), projectID); err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.WriteJSON(browserGUIServerMsg{Type: "starting"})

	project, err := s.projects.StartBrowserGUI(r.Context(), projectID)
	if err != nil {
		_ = conn.WriteJSON(browserGUIServerMsg{Type: "error", Message: err.Error()})
		return
	}
	_ = conn.WriteJSON(browserGUIServerMsg{Type: "ready", Slug: project.Slug, Port: browserGUINoVNCPort})

	// Hold the connection open for lifecycle control. We only act on an
	// explicit stop; a plain disconnect leaves the GUI running (see note
	// above) and is the common case (the user just closes the drawer).
	conn.SetReadLimit(1 << 16)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg browserGUIClientMsg
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.Type == "stop" {
			if err := s.projects.StopBrowserGUI(r.Context(), projectID); err != nil {
				_ = conn.WriteJSON(browserGUIServerMsg{Type: "error", Message: err.Error()})
				continue
			}
			_ = conn.WriteJSON(browserGUIServerMsg{Type: "stopped"})
		}
	}
}
