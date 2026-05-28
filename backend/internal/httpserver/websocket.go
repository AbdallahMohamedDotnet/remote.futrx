package httpserver

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func NewUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		// Existing deployments rely on edge auth / same-origin enforcement.
		CheckOrigin: func(*http.Request) bool { return true },
	}
}
