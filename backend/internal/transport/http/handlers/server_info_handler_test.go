package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	serviceserverinfo "github.com/futrx-com/remote.futrx.com/internal/service/serverinfo"
)

type stubServerInfoService struct {
	info serviceserverinfo.Info
}

func (s stubServerInfoService) Collect(context.Context) serviceserverinfo.Info {
	return s.info
}

func TestServerInfoHandler(t *testing.T) {
	usage := 31.5
	handler := NewServerInfoHandler(stubServerInfoService{
		info: serviceserverinfo.Info{
			CollectedAt: 123,
			Host:        serviceserverinfo.HostInfo{Hostname: "parent-1"},
			CPU:         serviceserverinfo.CPUInfo{LogicalCores: 8, UsagePercent: &usage},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/server/info", nil)
	response := httptest.NewRecorder()
	handler.HandleInfo(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body serviceserverinfo.Info
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Host.Hostname != "parent-1" || body.CPU.LogicalCores != 8 {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestServerInfoHandlerRejectsNonGET(t *testing.T) {
	handler := NewServerInfoHandler(stubServerInfoService{})
	request := httptest.NewRequest(http.MethodPost, "/api/server/info", nil)
	response := httptest.NewRecorder()

	handler.HandleInfo(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
