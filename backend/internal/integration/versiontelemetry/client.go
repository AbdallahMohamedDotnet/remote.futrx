// Package versiontelemetry reports a privacy-limited application-version
// heartbeat to remote.futrx.com. It sends only a random pseudonymous
// installation identity and the embedded application version.
package versiontelemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	endpointURL           = "https://remote.futrx.com/api/telemetry/version"
	requestTimeout        = 3 * time.Second
	minimumReportInterval = 7 * 24 * time.Hour
	identityBytes         = 16
	identityLength        = identityBytes * 2
)

var (
	errRedirectRejected      = errors.New("version telemetry redirect rejected")
	errInvalidInstallationID = errors.New("invalid installation identity")
	errInvalidReportState    = errors.New("invalid version telemetry state")
)

type reportState struct {
	Version         string `json:"version"`
	LastAttemptUnix int64  `json:"lastAttemptUnix"`
}

type Client struct {
	httpClient   *http.Client
	endpoint     string
	identityPath string
	statePath    string
	random       io.Reader
	now          func() time.Time

	mu             sync.Mutex
	installationID string
	stateMu        sync.Mutex
}

func New(dataDir string) *Client {
	return newClient(
		dataDir,
		endpointURL,
		newHTTPClient(requestTimeout),
		rand.Reader,
	)
}

func newClient(dataDir, endpoint string, httpClient *http.Client, random io.Reader) *Client {
	return &Client{
		httpClient:   httpClient,
		endpoint:     endpoint,
		identityPath: filepath.Join(dataDir, "telemetry", "installation-id"),
		statePath:    filepath.Join(dataDir, "telemetry", "version-report-state.json"),
		random:       random,
		now:          time.Now,
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirectRejected
		},
	}
}

func (c *Client) ReportVersion(ctx context.Context, version string) error {
	installationID, err := c.loadOrCreateInstallationID()
	if err != nil {
		return fmt.Errorf("load installation identity: %w", err)
	}
	due, err := c.reserveReportAttempt(version, c.now())
	if err != nil {
		return fmt.Errorf("reserve version telemetry attempt: %w", err)
	}
	if !due {
		return nil
	}
	payload, err := json.Marshal(struct {
		InstallationID string `json:"installationId"`
		Version        string `json:"version"`
	}{InstallationID: installationID, Version: version})
	if err != nil {
		return fmt.Errorf("encode version telemetry: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create version telemetry request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	// net/http otherwise adds Go's default user agent. The telemetry contract
	// intentionally sends no user-agent or host-identifying application data.
	request.Header["User-Agent"] = nil

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send version telemetry: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("send version telemetry: HTTP %d", response.StatusCode)
	}
	return nil
}

// reserveReportAttempt enforces an at-most-weekly request for one version
// across process restarts. A changed version is always due immediately. The
// attempt is persisted before network I/O so a failed request or process crash
// cannot create a restart-driven retry loop.
func (c *Client) reserveReportAttempt(version string, now time.Time) (bool, error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	state, err := readReportState(c.statePath)
	if err == nil && state.Version == version {
		lastAttempt := time.Unix(state.LastAttemptUnix, 0)
		elapsed := now.Sub(lastAttempt)
		if elapsed >= 0 && elapsed < minimumReportInterval {
			return false, nil
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, errInvalidReportState) {
		return false, err
	}

	encoded, err := json.Marshal(reportState{
		Version:         version,
		LastAttemptUnix: now.Unix(),
	})
	if err != nil {
		return false, err
	}
	if err := writePrivateFile(c.statePath, encoded); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) loadOrCreateInstallationID() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.installationID != "" {
		return c.installationID, nil
	}

	installationID, err := readInstallationID(c.identityPath)
	if err == nil {
		c.installationID = installationID
		return installationID, nil
	}
	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, errInvalidInstallationID) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(c.identityPath), 0o700); err != nil {
		return "", err
	}

	randomBytes := make([]byte, identityBytes)
	if _, err := io.ReadFull(c.random, randomBytes); err != nil {
		return "", err
	}
	installationID = hex.EncodeToString(randomBytes)
	if err := writeInstallationID(c.identityPath, installationID); err != nil {
		return "", err
	}
	c.installationID = installationID
	return installationID, nil
}

func writeInstallationID(path, installationID string) error {
	return writePrivateFile(path, []byte(installationID))
}

func writePrivateFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".telemetry-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath) // no-op after a successful rename

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func readReportState(path string) (reportState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return reportState{}, err
	}
	var state reportState
	if err := json.Unmarshal(raw, &state); err != nil || state.Version == "" || state.LastAttemptUnix <= 0 {
		return reportState{}, errInvalidReportState
	}
	return state, nil
}

func readInstallationID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	installationID := string(raw)
	if len(installationID) != identityLength {
		return "", fmt.Errorf("%w: invalid length", errInvalidInstallationID)
	}
	for _, character := range installationID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", fmt.Errorf("%w: invalid format", errInvalidInstallationID)
		}
	}
	return installationID, nil
}
