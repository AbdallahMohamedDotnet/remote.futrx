package minimax

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

func TestAPIKeyValidatorAcceptsAuthenticatedModelCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer valid-key" {
			t.Errorf("Authorization = %q", got)
			return
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	validator := &apiKeyValidator{client: server.Client(), endpoint: server.URL + "/v1/models"}
	if err := validator.ValidateAPIKey(context.Background(), "valid-key"); err != nil {
		t.Fatal(err)
	}
}

func TestAPIKeyValidatorRejectsUnauthorizedKeysWithoutEchoingProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"body-must-not-escape"}`))
	}))
	defer server.Close()

	validator := &apiKeyValidator{client: server.Client(), endpoint: server.URL}
	err := validator.ValidateAPIKey(context.Background(), "rejected-key")
	if !errors.Is(err, agentauth.ErrAPIKeyRejected) {
		t.Fatalf("error = %v, want ErrAPIKeyRejected", err)
	}
	if strings.Contains(err.Error(), "body-must-not-escape") {
		t.Fatalf("provider body escaped into error: %v", err)
	}
}

func TestAPIKeyValidatorTreatsProviderAndPayloadFailuresAsTemporary(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "provider error", status: http.StatusInternalServerError, body: `{"error":"internal"}`},
		{name: "malformed JSON", status: http.StatusOK, body: `{`},
		{name: "wrong shape", status: http.StatusOK, body: `{"object":"other","data":[]}`},
		{name: "missing data", status: http.StatusOK, body: `{"object":"list"}`},
		{
			name:   "oversized response",
			status: http.StatusOK,
			body:   strings.Repeat("x", maxAPIKeyValidationResponseBytes+1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			validator := &apiKeyValidator{client: server.Client(), endpoint: server.URL}
			if err := validator.ValidateAPIKey(context.Background(), "key"); !errors.Is(err, ErrAPIKeyValidationUnavailable) {
				t.Fatalf("error = %v, want ErrAPIKeyValidationUnavailable", err)
			}
		})
	}
}

func TestAPIKeyValidatorHonorsHTTPClientTimeout(t *testing.T) {
	client := &http.Client{
		Timeout: 20 * time.Millisecond,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}
	validator := &apiKeyValidator{client: client, endpoint: "https://api.minimax.invalid/v1/models"}
	if err := validator.ValidateAPIKey(context.Background(), "key"); !errors.Is(err, ErrAPIKeyValidationUnavailable) {
		t.Fatalf("error = %v, want ErrAPIKeyValidationUnavailable", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
