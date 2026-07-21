package wstransport

import (
	"context"
	"net/http"
	"time"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/workspacehub"
	"github.com/gorilla/websocket"
)

type ChatLister interface {
	List(ctx context.Context) ([]servicechat.Meta, error)
}

type ProjectLister interface {
	List(ctx context.Context) ([]serviceproject.Meta, error)
}

// WorkspaceVisibility filters the initial snapshot so users only see chats
// + projects they can reach. Provided by the auth wiring; if nil, the
// snapshot is unfiltered (single-user dev mode).
type WorkspaceVisibility interface {
	CallerAndAdmin(ctx context.Context, r *http.Request) (string, bool, error)
	HasAccess(ctx context.Context, projectID serviceproject.ID, email string) (bool, error)
}

type WorkspaceSocket struct {
	chats      ChatLister
	projects   ProjectLister
	hub        *workspacehub.Hub
	visibility WorkspaceVisibility
}

type workspaceSnapshot struct {
	Type     string                `json:"type"`
	Chats    []servicechat.Meta    `json:"chats"`
	Projects []serviceproject.Meta `json:"projects"`
}

func NewWorkspaceSocket(chats ChatLister, projects ProjectLister, hub *workspacehub.Hub) *WorkspaceSocket {
	return &WorkspaceSocket{chats: chats, projects: projects, hub: hub}
}

func (s *WorkspaceSocket) WithVisibility(v WorkspaceVisibility) *WorkspaceSocket {
	s.visibility = v
	return s
}

func (s *WorkspaceSocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/workspace", s.Handle(upgrader))
}

func (s *WorkspaceSocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

func (s *WorkspaceSocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		http.Error(w, "workspace stream unavailable", http.StatusServiceUnavailable)
		return
	}

	email, isAdmin := "", true
	if s.visibility != nil {
		em, admin, err := s.visibility.CallerAndAdmin(r.Context(), r)
		if err != nil || em == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		email, isAdmin = em, admin
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 16)

	sub := s.hub.Subscribe()
	defer sub.Close()

	chats, err := s.chats.List(r.Context())
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"type": "error", "message": err.Error()})
		return
	}
	projects, err := s.projects.List(r.Context())
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"type": "error", "message": err.Error()})
		return
	}

	if s.visibility != nil && !isAdmin {
		projects = s.filterProjects(r.Context(), projects, email)
		allowed := projectIDSet(projects)
		chats = s.filterChats(chats, allowed)
	}

	done := make(chan struct{})
	go func() {
		defer conn.Close()
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		if err := conn.WriteJSON(workspaceSnapshot{
			Type:     "workspace.snapshot",
			Chats:    chats,
			Projects: projects,
		}); err != nil {
			return
		}

		for {
			select {
			case ev, ok := <-sub.Events():
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if err := conn.WriteJSON(ev); err != nil {
					return
				}
			case <-ticker.C:
				deadline := time.Now().Add(15 * time.Second)
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			close(done)
			return
		}
	}
}

func (s *WorkspaceSocket) filterProjects(ctx context.Context, in []serviceproject.Meta, email string) []serviceproject.Meta {
	out := make([]serviceproject.Meta, 0, len(in))
	for _, p := range in {
		ok, err := s.visibility.HasAccess(ctx, p.ID, email)
		if err == nil && ok {
			out = append(out, p)
		}
	}
	return out
}

func (s *WorkspaceSocket) filterChats(in []servicechat.Meta, allowedProjects map[serviceproject.ID]struct{}) []servicechat.Meta {
	out := make([]servicechat.Meta, 0, len(in))
	for _, c := range in {
		if c.ProjectID == "" {
			out = append(out, c)
			continue
		}
		if _, ok := allowedProjects[serviceproject.ID(c.ProjectID)]; ok {
			out = append(out, c)
		}
	}
	return out
}

func projectIDSet(projects []serviceproject.Meta) map[serviceproject.ID]struct{} {
	m := make(map[serviceproject.ID]struct{}, len(projects))
	for _, p := range projects {
		m[p.ID] = struct{}{}
	}
	return m
}
