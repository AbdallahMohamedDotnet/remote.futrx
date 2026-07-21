package httphandlers

import (
	"context"
	"net/http"

	serviceserverinfo "github.com/futrx-com/remote.futrx.com/internal/service/serverinfo"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

type ServerInfoService interface {
	Collect(context.Context) serviceserverinfo.Info
}

type ServerInfoHandler struct {
	server ServerInfoService
}

func NewServerInfoHandler(server ServerInfoService) *ServerInfoHandler {
	return &ServerInfoHandler{server: server}
}

func (h *ServerInfoHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/server/info", h.HandleInfo)
}

func (h *ServerInfoHandler) HandleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	httptransport.SendJSON(w, http.StatusOK, h.server.Collect(r.Context()))
}
