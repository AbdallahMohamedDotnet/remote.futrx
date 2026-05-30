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

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	"github.com/gorilla/websocket"
)

type ChatLookup interface {
	Get(ctx context.Context, id servicechat.ID) (servicechat.Meta, error)
}

type PromptRunner interface {
	StartPrompt(id servicechat.ID, prompt string, emitTransient func(servicechat.Event))
	CancelPrompt(id servicechat.ID) bool
}

type ChatSocket struct {
	chats  ChatLookup
	hub    *runhub.Hub
	runner PromptRunner
}

func NewChatSocket(chats ChatLookup, hub *runhub.Hub, runner PromptRunner) *ChatSocket {
	return &ChatSocket{chats: chats, hub: hub, runner: runner}
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
	if _, err := s.chats.Get(r.Context(), id); err != nil {
		if errors.Is(err, servicechat.ErrNotFound) || os.IsNotExist(err) {
			http.Error(w, "chat not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
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
