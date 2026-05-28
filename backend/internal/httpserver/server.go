package httpserver

import (
	"net/http"
	"time"
)

type AuthRoutes struct {
	Login      http.HandlerFunc
	Callback   http.HandlerFunc
	Logout     http.HandlerFunc
	Me         http.HandlerFunc
	Middleware func(http.Handler) http.Handler
}

type Routes struct {
	Sessions        http.HandlerFunc
	SessionResource http.HandlerFunc
	Chats           http.HandlerFunc
	ChatResource    http.HandlerFunc
	ClaudeAuth      http.HandlerFunc
	ClaudeLogin     http.HandlerFunc
	ClaudeCode      http.HandlerFunc
	ClaudeCancel    http.HandlerFunc
	TmuxWS          http.HandlerFunc
	ChatWS          http.HandlerFunc
	Auth            *AuthRoutes
	Static          http.Handler
}

func NewHandler(routes Routes) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", routes.Sessions)
	mux.HandleFunc("/api/sessions/", routes.SessionResource)
	mux.HandleFunc("/api/chats", routes.Chats)
	mux.HandleFunc("/api/chats/", routes.ChatResource)
	mux.HandleFunc("/api/claude/auth-status", routes.ClaudeAuth)
	mux.HandleFunc("/api/claude/login/start", routes.ClaudeLogin)
	mux.HandleFunc("/api/claude/login/code", routes.ClaudeCode)
	mux.HandleFunc("/api/claude/login/cancel", routes.ClaudeCancel)
	mux.HandleFunc("/ws", routes.TmuxWS)
	mux.HandleFunc("/ws/chat/", routes.ChatWS)
	if routes.Auth != nil {
		mux.HandleFunc("/auth/google/login", routes.Auth.Login)
		mux.HandleFunc("/auth/google/callback", routes.Auth.Callback)
		mux.HandleFunc("/auth/logout", routes.Auth.Logout)
		mux.HandleFunc("/auth/me", routes.Auth.Me)
	}
	mux.Handle("/", routes.Static)

	var handler http.Handler = mux
	if routes.Auth != nil && routes.Auth.Middleware != nil {
		handler = routes.Auth.Middleware(mux)
	}
	return handler
}

func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: long-lived WebSockets.
	}
}
