package wstransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/runhub"
	"github.com/gorilla/websocket"
)

type ChatLookup interface {
	Get(ctx context.Context, id servicechat.ID) (servicechat.Meta, error)
}

type PromptRunner interface {
	StartPrompt(id servicechat.ID, prompt string, emitTransient func(servicechat.Event))
	CancelPrompt(id servicechat.ID) bool
}

// ProjectAccessChecker is the subset of the auth gate the chat WS needs:
// resolve the caller from the request and verify they can reach a project.
// Implemented by a small adapter wired up in transport.go.
type ProjectAccessChecker interface {
	CallerAndAdmin(ctx context.Context, r *http.Request) (string, bool, error)
	HasAccess(ctx context.Context, projectID serviceproject.ID, email string) (bool, error)
}

type ChatSocket struct {
	chats  ChatLookup
	hub    *runhub.Hub
	runner PromptRunner
	access ProjectAccessChecker
}

func NewChatSocket(chats ChatLookup, hub *runhub.Hub, runner PromptRunner) *ChatSocket {
	return &ChatSocket{chats: chats, hub: hub, runner: runner}
}

// WithAccessChecker turns on per-chat project-membership gating.
func (s *ChatSocket) WithAccessChecker(access ProjectAccessChecker) *ChatSocket {
	s.access = access
	return s
}

func (s *ChatSocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

func (s *ChatSocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/chat/", s.Handle(upgrader))
}

func (s *ChatSocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	id := servicechat.ID(strings.TrimPrefix(r.URL.Path, "/ws/chat/"))
	if !servicechat.ValidID(id) {
		http.Error(w, "invalid chat id", http.StatusBadRequest)
		return
	}
	meta, err := s.chats.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, servicechat.ErrNotFound) || os.IsNotExist(err) {
			http.Error(w, "chat not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if s.access != nil && meta.ProjectID != "" {
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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)

	sub, err := s.hub.SubscribeAfter(r.Context(), id, sinceSeq(r))
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sub.Close()

	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		defer conn.Close()

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
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "prompt":
			s.runner.StartPrompt(id, msg.Text, sub.SendTransient)
		case "cancel":
			if !s.runner.CancelPrompt(id) {
				sub.SendTransient(servicechat.Event{
					T:       time.Now().UnixMilli(),
					Type:    "error",
					Message: "no prompt is currently running",
				})
			}
		}
	}
}

func sinceSeq(r *http.Request) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get("since"))
	if raw == "" {
		return 0
	}
	seq, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seq < 0 {
		return 0
	}
	return seq
}
