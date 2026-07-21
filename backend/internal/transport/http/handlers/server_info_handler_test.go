package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerInfoHandler(t *testing.T) {
	handler := NewServerInfoHandler(t.TempDir())
	handler.collect = func(context.Context) ServerInfo {
		usage := 31.5
		return ServerInfo{
			CollectedAt: 123,
			Host:        ServerHostInfo{Hostname: "parent-1"},
			CPU:         ServerCPUInfo{LogicalCores: 8, UsagePercent: &usage},
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/server/info", nil)
	response := httptest.NewRecorder()
	handler.HandleInfo(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body ServerInfo
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Host.Hostname != "parent-1" || body.CPU.LogicalCores != 8 {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestServerInfoHandlerRejectsNonGET(t *testing.T) {
	handler := NewServerInfoHandler(t.TempDir())
	request := httptest.NewRequest(http.MethodPost, "/api/server/info", nil)
	response := httptest.NewRecorder()

	handler.HandleInfo(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestParseMemInfo(t *testing.T) {
	values := parseMemInfo("MemTotal:       1000 kB\nMemAvailable:    400 kB\nSwapTotal:       200 kB\n")
	if values["MemTotal"] != 1000*1024 || values["MemAvailable"] != 400*1024 {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestReadNetworkCounters(t *testing.T) {
	values := readNetworkCounters(`Inter-| Receive | Transmit
 eth0: 1024 1 2 3 4 5 6 7 2048 9 10 11 12 13 14 15
    lo:  100 1 2 3 4 5 6 7  100 9 10 11 12 13 14 15`)
	if values["eth0"] != [2]uint64{1024, 2048} {
		t.Fatalf("unexpected counters: %#v", values)
	}
}

func TestCPUUsage(t *testing.T) {
	got := cpuUsage(cpuSample{total: 100, idle: 40}, cpuSample{total: 200, idle: 70})
	if got != 70 {
		t.Fatalf("usage = %v, want 70", got)
	}
}
