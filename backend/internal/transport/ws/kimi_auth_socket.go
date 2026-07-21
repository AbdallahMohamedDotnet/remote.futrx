package wstransport

import (
	"net/http"
	"time"

	kimiagent "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/kimi"
	"github.com/gorilla/websocket"
)

type KimiAuthSocket struct {
	login *kimiagent.Auth
}

func NewKimiAuthSocket(login *kimiagent.Auth) *KimiAuthSocket {
	return &KimiAuthSocket{login: login}
}

func (s *KimiAuthSocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/kimi/auth-status", s.Handle(upgrader))
}

func (s *KimiAuthSocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

func (s *KimiAuthSocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	if s.login == nil {
		http.Error(w, "kimi auth stream unavailable", http.StatusServiceUnavailable)
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
