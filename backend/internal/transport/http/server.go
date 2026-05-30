package httptransport

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type RouteRegistrar interface {
	RegisterRoutes(*http.ServeMux)
}

type WebSocketRegistrar interface {
	RegisterRoutes(*http.ServeMux, websocket.Upgrader)
}

type AuthRegistrar interface {
	RouteRegistrar
	Middleware(http.Handler) http.Handler
}

type Handlers struct {
	Sessions   RouteRegistrar
	Chats      RouteRegistrar
	Projects   RouteRegistrar
	ClaudeAuth RouteRegistrar
	TmuxWS     WebSocketRegistrar
	ChatWS     WebSocketRegistrar
	Auth       AuthRegistrar
	Static     http.Handler
}

func NewHandler(handlers Handlers) http.Handler {
	mux := http.NewServeMux()

	register := func(handler RouteRegistrar) {
		if handler != nil {
			handler.RegisterRoutes(mux)
		}
	}

	register(handlers.Sessions)
	register(handlers.Chats)
	register(handlers.Projects)
	register(handlers.ClaudeAuth)

	upgrader := NewUpgrader()
	if handlers.TmuxWS != nil {
		handlers.TmuxWS.RegisterRoutes(mux, upgrader)
	}
	if handlers.ChatWS != nil {
		handlers.ChatWS.RegisterRoutes(mux, upgrader)
	}
	if handlers.Auth != nil {
		handlers.Auth.RegisterRoutes(mux)
	}
	if handlers.Static != nil {
		mux.Handle("/", handlers.Static)
	}

	var handler http.Handler = mux
	if handlers.Auth != nil {
		handler = handlers.Auth.Middleware(handler)
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
