package wstransport

import (
	"net/http"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/codexauth"
	"github.com/gorilla/websocket"
)

type CodexAuthSocket struct {
	login *codexauth.Manager
}

func NewCodexAuthSocket(login *codexauth.Manager) *CodexAuthSocket {
	return &CodexAuthSocket{login: login}
}

func (s *CodexAuthSocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/codex/auth-status", s.Handle(upgrader))
}

func (s *CodexAuthSocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

func (s *CodexAuthSocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	if s.login == nil {
		http.Error(w, "codex auth stream unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)

	statuses, unsubscribe := s.login.Subscribe()
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer conn.Close()
		for {
			select {
			case status, ok := <-statuses:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if err := conn.WriteJSON(status); err != nil {
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
