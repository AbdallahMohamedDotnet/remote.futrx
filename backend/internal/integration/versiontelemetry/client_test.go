package versiontelemetry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReportVersionPersistsIdentityAndPostsMinimalPayload(t *testing.T) {
	type requestRecord struct {
		method      string
		path        string
		contentType string
		userAgent   bool
		body        string
	}
	records := make(chan requestRecord, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasUserAgent := r.Header["User-Agent"]
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		records <- requestRecord{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			userAgent:   hasUserAgent,
			body:        string(body),
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	first := newClient(dataDir, server.URL+"/api/telemetry/version", server.Client(), bytes.NewReader(bytes.Repeat([]byte{0xab}, identityBytes)))
	if err := first.ReportVersion(context.Background(), "42989fe"); err != nil {
		t.Fatal(err)
	}
	// A new client simulates a process restart. Different entropy must not
	// replace the identity already persisted by the first process.
	second := newClient(dataDir, server.URL+"/api/telemetry/version", server.Client(), bytes.NewReader(bytes.Repeat([]byte{0xcd}, identityBytes)))
	if err := second.ReportVersion(context.Background(), "0.21.0"); err != nil {
		t.Fatal(err)
	}

	wantID := strings.Repeat("ab", identityBytes)
	wantBodies := []string{
		`{"installationId":"` + wantID + `","version":"42989fe"}`,
		`{"installationId":"` + wantID + `","version":"0.21.0"}`,
	}
	for index, wantBody := range wantBodies {
		record := <-records
		if record.method != http.MethodPost || record.path != "/api/telemetry/version" {
			t.Fatalf("request %d = %s %s", index, record.method, record.path)
		}
		if record.contentType != "application/json" {
			t.Fatalf("request %d content type = %q", index, record.contentType)
		}
		if record.userAgent {
			t.Fatalf("request %d unexpectedly included User-Agent", index)
		}
		if record.body != wantBody {
			t.Fatalf("request %d body = %q, want %q", index, record.body, wantBody)
		}
	}

	identityPath := filepath.Join(dataDir, "telemetry", "installation-id")
	raw, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != wantID {
		t.Fatalf("persisted identity = %q, want %q", raw, wantID)
	}
	info, err := os.Stat(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("identity mode = %o, want 600", got)
	}
}

func TestReportVersionRejectsNonSuccessStatus(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	client := newClient(dataDir, server.URL, server.Client(), bytes.NewReader(make([]byte, identityBytes)))
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }
	err := client.ReportVersion(context.Background(), "0.21.0")
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("error = %v, want HTTP 503", err)
	}

	// A failed attempt is still recorded before network I/O, preventing an
	// ordinary restart from turning a collector outage into repeated requests.
	restarted := newClient(dataDir, server.URL, server.Client(), bytes.NewReader(make([]byte, identityBytes)))
	restarted.now = func() time.Time { return now.Add(24 * time.Hour) }
	if err := restarted.ReportVersion(context.Background(), "0.21.0"); err != nil {
		t.Fatalf("suppressed retry: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestReportVersionReplacesInvalidPersistedIdentity(t *testing.T) {
	tests := []struct {
		name     string
		existing string
	}{
		{name: "truncated", existing: "abc123"},
		{name: "invalid characters", existing: strings.Repeat("G", identityLength)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read request: %v", err)
				}
				body = string(raw)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			dataDir := t.TempDir()
			identityDir := filepath.Join(dataDir, "telemetry")
			if err := os.MkdirAll(identityDir, 0o700); err != nil {
				t.Fatal(err)
			}
			identityPath := filepath.Join(identityDir, "installation-id")
			if err := os.WriteFile(identityPath, []byte(test.existing), 0o600); err != nil {
				t.Fatal(err)
			}

			client := newClient(dataDir, server.URL, server.Client(), bytes.NewReader(bytes.Repeat([]byte{0xcd}, identityBytes)))
			if err := client.ReportVersion(context.Background(), "0.21.0"); err != nil {
				t.Fatal(err)
			}

			wantID := strings.Repeat("cd", identityBytes)
			wantBody := `{"installationId":"` + wantID + `","version":"0.21.0"}`
			if body != wantBody {
				t.Fatalf("body = %q, want %q", body, wantBody)
			}
			raw, err := os.ReadFile(identityPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != wantID {
				t.Fatalf("replacement identity = %q, want %q", raw, wantID)
			}
			info, err := os.Stat(identityPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("replacement identity mode = %o, want 600", got)
			}
			entries, err := os.ReadDir(identityDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 || entries[0].Name() != "installation-id" || entries[1].Name() != "version-report-state.json" {
				t.Fatalf("identity directory entries = %v, want identity and report state", entries)
			}
		})
	}
}

func TestReportVersionIsWeeklyAcrossRestartsAndImmediateForNewVersion(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		requests <- string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	start := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	newReporter := func(now time.Time) *Client {
		client := newClient(dataDir, server.URL, server.Client(), bytes.NewReader(bytes.Repeat([]byte{0xab}, identityBytes)))
		client.now = func() time.Time { return now }
		return client
	}

	if err := newReporter(start).ReportVersion(context.Background(), "0.21.0"); err != nil {
		t.Fatal(err)
	}
	if err := newReporter(start.Add(24*time.Hour)).ReportVersion(context.Background(), "0.21.0"); err != nil {
		t.Fatal(err)
	}
	if err := newReporter(start.Add(48*time.Hour)).ReportVersion(context.Background(), "0.21.1"); err != nil {
		t.Fatal(err)
	}
	if err := newReporter(start.Add(8*24*time.Hour)).ReportVersion(context.Background(), "0.21.1"); err != nil {
		t.Fatal(err)
	}
	if err := newReporter(start.Add(9*24*time.Hour)).ReportVersion(context.Background(), "0.21.1"); err != nil {
		t.Fatal(err)
	}

	wantVersions := []string{"0.21.0", "0.21.1", "0.21.1"}
	for _, version := range wantVersions {
		select {
		case body := <-requests:
			if !strings.Contains(body, `"version":"`+version+`"`) {
				t.Fatalf("body = %q, want version %s", body, version)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for version %s", version)
		}
	}
	select {
	case body := <-requests:
		t.Fatalf("unexpected extra telemetry request: %s", body)
	default:
	}

	statePath := filepath.Join(dataDir, "telemetry", "version-report-state.json")
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("report state mode = %o, want 600", got)
	}
}

func TestReportVersionRejectsRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	client := newClient(t.TempDir(), server.URL, newHTTPClient(time.Second), bytes.NewReader(make([]byte, identityBytes)))
	err := client.ReportVersion(context.Background(), "0.21.0")
	if err == nil || !errors.Is(err, errRedirectRejected) {
		t.Fatalf("error = %v, want redirect rejection", err)
	}
	if redirected.Load() != 0 {
		t.Fatal("telemetry client followed redirect")
	}
}

func TestReportVersionHonorsShortHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newClient(t.TempDir(), server.URL, newHTTPClient(20*time.Millisecond), bytes.NewReader(make([]byte, identityBytes)))
	started := time.Now()
	err := client.ReportVersion(context.Background(), "0.21.0")
	if err == nil {
		t.Fatal("timeout returned no error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout took %s, want under 1s", elapsed)
	}
}
