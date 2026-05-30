package wstransport

import (
	"context"
	"net/http"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/workspacehub"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/gorilla/websocket"
)

type ChatLister interface {
	List(ctx context.Context) ([]servicechat.Meta, error)
}

type ProjectLister interface {
	List(ctx context.Context) ([]serviceproject.Meta, error)
}

type WorkspaceSocket struct {
	chats    ChatLister
	projects ProjectLister
	hub      *workspacehub.Hub
}

type workspaceSnapshot struct {
	Type     string                `json:"type"`
	Chats    []servicechat.Meta    `json:"chats"`
	Projects []serviceproject.Meta `json:"projects"`
}

func NewWorkspaceSocket(chats ChatLister, projects ProjectLister, hub *workspacehub.Hub) *WorkspaceSocket {
	return &WorkspaceSocket{chats: chats, projects: projects, hub: hub}
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
