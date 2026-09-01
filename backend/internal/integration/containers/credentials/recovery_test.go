package credentials_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/claude"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/credentials"
)

const (
	hostOAuth      = `{"claudeAiOauth":{"accessToken":"expired-host","refreshToken":"host-refresh","expiresAt":1}}`
	containerOAuth = `{"claudeAiOauth":{"accessToken":"expired-project","refreshToken":"project-refresh","expiresAt":1}}`
	clearedOAuth   = `{"claudeAiOauth":{"scopes":["user:inference"]}}`
)

func TestClaudeCredentialRecovery(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		container    string
		containerAge time.Duration
		missing      bool
		readErr      error
		wantPush     bool
		wantErr      bool
	}{
		{name: "cleared newer project recovers", host: hostOAuth, container: clearedOAuth, containerAge: time.Minute, wantPush: true},
		{name: "cleared equal timestamp recovers", host: hostOAuth, container: clearedOAuth, wantPush: true},
		{name: "malformed newer project recovers", host: hostOAuth, container: `{"claudeAiOauth":`, containerAge: time.Minute, wantPush: true},
		{name: "expired unrefreshable project recovers", host: hostOAuth, container: `{"claudeAiOauth":{"accessToken":"expired","expiresAt":1}}`, containerAge: time.Minute, wantPush: true},
		{name: "missing project credentials seeded", host: hostOAuth, missing: true, wantPush: true},
		{name: "newer refreshable project preserved", host: hostOAuth, container: containerOAuth, containerAge: time.Minute},
		{name: "equal refreshable project preserved", host: hostOAuth, container: containerOAuth},
		{name: "new host login copied", host: hostOAuth, container: containerOAuth, containerAge: -time.Minute, wantPush: true},
		{name: "cleared newer host cannot erase project", host: clearedOAuth, container: containerOAuth, containerAge: -time.Minute},
		{name: "malformed newer host cannot erase project", host: `{`, container: containerOAuth, containerAge: -time.Minute},
		{name: "unusable host cannot repair project", host: clearedOAuth, container: clearedOAuth, containerAge: time.Minute},
		{name: "read failure preserves project", host: hostOAuth, container: containerOAuth, containerAge: time.Minute, readErr: errors.New("read unavailable"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := claudeCredentialSpec(t, tt.host)
			hostTime := time.Unix(1_700_000_000, 0)
			if err := os.Chtimes(spec.Files[0].HostPath, hostTime, hostTime); err != nil {
				t.Fatal(err)
			}
			runner := &credentialRunner{
				data: tt.container, modified: hostTime.Add(tt.containerAge),
				missing: tt.missing, readErr: tt.readErr,
			}
			err := credentials.NewAdapter(runner).EnsureFiles(context.Background(), "project", spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureFiles error = %v, want error %v", err, tt.wantErr)
			}
			if (runner.pushes > 0) != tt.wantPush {
				t.Fatalf("push count = %d, want push %v", runner.pushes, tt.wantPush)
			}
			want := tt.container
			if tt.wantPush {
				want = tt.host
			}
			if runner.data != want {
				t.Fatal("container did not retain the expected credential document")
			}
		})
	}
}

func TestClaudeCredentialPullPreservesHostOnInvalidOrFailedTransfer(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		err  error
	}{
		{"cleared credentials", clearedOAuth, nil},
		{"partial JSON", `{"claudeAiOauth":{"refreshToken":"private-marker"`, nil},
		{"failed transfer", containerOAuth, errors.New("transfer failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := claudeCredentialSpec(t, hostOAuth)
			hostPath := spec.Files[0].HostPath
			before, err := os.Stat(hostPath)
			if err != nil {
				t.Fatal(err)
			}
			runner := &credentialRunner{data: tt.data, pullErr: tt.err}
			err = credentials.NewAdapter(runner).SyncFilesFromContainer(context.Background(), "project", spec)
			if err == nil {
				t.Fatal("expected rejected credential transfer")
			}
			if strings.Contains(err.Error(), "private-marker") || strings.Contains(err.Error(), "project-refresh") {
				t.Fatal("credential data leaked in error")
			}
			data, err := os.ReadFile(hostPath)
			if err != nil || string(data) != hostOAuth {
				t.Fatalf("host credentials changed after failed transfer: %v", err)
			}
			after, err := os.Stat(hostPath)
			if err != nil || !after.ModTime().Equal(before.ModTime()) {
				t.Fatalf("host timestamp changed after failed transfer: %v", err)
			}
			assertNoCredentialTemporaryFiles(t, spec.HostDir)
		})
	}
}

func TestClaudeCredentialPullRetainsRefreshAndSeedsAnotherProject(t *testing.T) {
	spec := claudeCredentialSpec(t, hostOAuth)
	runner := &credentialRunner{data: containerOAuth}
	adapter := credentials.NewAdapter(runner)
	if err := adapter.SyncFilesFromContainer(context.Background(), "project", spec); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(spec.Files[0].HostPath)
	if err != nil || string(data) != containerOAuth {
		t.Fatalf("refreshed credentials were not retained on host: %v", err)
	}
	info, err := os.Stat(spec.Files[0].HostPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("host credential permissions are not private: %v", err)
	}
	assertNoCredentialTemporaryFiles(t, spec.HostDir)
	// A failed refresh rewrites the other project's file later than the host.
	runner.data = clearedOAuth
	runner.modified = time.Now().Add(time.Minute)
	if err := adapter.EnsureFiles(context.Background(), "other-project", spec); err != nil {
		t.Fatal(err)
	}
	if runner.pushes != 1 || runner.data != containerOAuth {
		t.Fatal("new host credentials did not repair the other project")
	}
}

func claudeCredentialSpec(t *testing.T, data string) provisioning.CredentialSpec {
	t.Helper()
	spec := claude.Profile().Credentials
	spec.HostDir = t.TempDir()
	spec.LegacyDevices = nil
	spec.Files = spec.Files[1:] // Exercise the actual Claude OAuth policy.
	spec.Files[0].HostPath = filepath.Join(spec.HostDir, ".credentials.json")
	if err := os.WriteFile(spec.Files[0].HostPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return spec
}

func assertNoCredentialTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".credentials.json" {
		t.Fatal("temporary credential file was not removed")
	}
}

// credentialRunner emulates only the LXD file operations, with real host I/O
// so tests check whether failed transfers actually preserve the saved login.
type credentialRunner struct {
	data     string
	modified time.Time
	missing  bool
	readErr  error
	pullErr  error
	pushes   int
}

func (*credentialRunner) Available() bool { return true }

func (r *credentialRunner) Run(_ context.Context, args ...string) (string, error) {
	if len(args) >= 4 && args[0] == "exec" && args[2] == "--" {
		switch args[3] {
		case "install":
			return "", nil
		case "stat", "test":
			if r.missing {
				return "", os.ErrNotExist
			}
			return strconv.FormatInt(r.modified.Unix(), 10), nil
		case "cat":
			return r.data, r.readErr
		}
	}
	if len(args) >= 4 && args[0] == "file" {
		switch args[1] {
		case "push":
			data, err := os.ReadFile(args[3])
			if err == nil {
				r.data = string(data)
				r.pushes++
			}
			return "", err
		case "pull":
			if err := os.WriteFile(args[3], []byte(r.data), 0o600); err != nil {
				return "", err
			}
			return r.data, r.pullErr
		}
	}
	return "", fmt.Errorf("unexpected command: %v", args)
}

func (r *credentialRunner) RunStdin(ctx context.Context, _ io.Reader, args ...string) (string, error) {
	return r.Run(ctx, args...)
}
