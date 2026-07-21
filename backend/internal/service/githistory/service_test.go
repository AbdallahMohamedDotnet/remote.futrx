package githistory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepositoryPath(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	makeGitRepo(t, workspaceRoot)
	makeGitRepo(t, filepath.Join(workspaceRoot, "packages", "app"))
	nestedRepo := filepath.Join(workspaceRoot, "packages", "app")
	service := New(testGitClient{})
	tests := []struct {
		name          string
		rawRepository string
		want          string
	}{
		{name: "root", rawRepository: ".", want: workspaceRoot},
		{name: "relative", rawRepository: "packages/app", want: nestedRepo},
		{name: "container", rawRepository: "/workspace/packages/app", want: nestedRepo},
		{name: "host", rawRepository: nestedRepo, want: nestedRepo},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := service.resolveRepositoryPath(test.rawRepository, workspaceRoot)
			if err != nil {
				t.Fatalf("resolveRepositoryPath() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("resolveRepositoryPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveRepositoryPathRejectsUnsafePaths(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	makeGitRepo(t, workspaceRoot)
	service := New(testGitClient{})
	tests := []string{"../other", "/workspace/../etc", filepath.Join(filepath.Dir(workspaceRoot), "other"), "/etc"}

	for _, rawRepository := range tests {
		t.Run(rawRepository, func(t *testing.T) {
			if got, err := service.resolveRepositoryPath(rawRepository, workspaceRoot); err == nil {
				t.Fatalf("resolveRepositoryPath() = %q, want error", got)
			}
		})
	}
}

func TestParseCommitLine(t *testing.T) {
	line := "0123456789abcdef\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f1712345678\x1fAdd history drawer"
	commit, err := parseCommitLine(line, "0123456789abcdef")
	if err != nil {
		t.Fatalf("parseCommitLine() error = %v", err)
	}
	if !commit.IsHead || commit.ShortSHA != "0123456" || commit.Subject != "Add history drawer" || commit.AuthorDate != 1712345678 {
		t.Fatalf("parseCommitLine() = %+v", commit)
	}
}

func TestParseDirtyFiles(t *testing.T) {
	status := " M app.tsx\n?? new-file.txt\nR  old.txt -> new.txt\n"
	files := parseDirtyFiles(status)
	want := []string{"M app.tsx", "?? new-file.txt", "R  old.txt -> new.txt"}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("parseDirtyFiles() = %#v, want %#v", files, want)
	}
}

func TestSanitizeCheckpointMessage(t *testing.T) {
	got := sanitizeCheckpointMessage("  checkpoint\n before\t switch  ")
	if got != "checkpoint before switch" {
		t.Fatalf("sanitizeCheckpointMessage() = %q", got)
	}
}

func TestCheckpointChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repository := t.TempDir()
	runTestGit(t, repository, "init")
	if err := os.WriteFile(filepath.Join(repository, "app.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "add", "app.txt")
	runTestGit(t, repository, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(repository, "app.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := New(testGitClient{})
	sha, err := service.checkpointChanges(context.Background(), repository, "  checkpoint\n before switch  ")
	if err != nil {
		t.Fatalf("checkpointChanges() error = %v", err)
	}
	if sha == "" {
		t.Fatal("checkpointChanges() returned empty sha")
	}
	status, dirtyFiles, err := service.repositoryStatus(context.Background(), repository)
	if err != nil {
		t.Fatalf("repositoryStatus() error = %v", err)
	}
	if status != "" || len(dirtyFiles) != 0 {
		t.Fatalf("status = %q, dirtyFiles = %#v; want clean", status, dirtyFiles)
	}
	message := runTestGit(t, repository, "log", "-1", "--pretty=%s")
	if strings.TrimSpace(message) != "checkpoint before switch" {
		t.Fatalf("commit subject = %q", message)
	}
}

type testGitClient struct{}

func (testGitClient) DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (testGitClient) IsRepository(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func (testGitClient) DiscoverRepositories(string, int, []string) []string {
	return nil
}

func (client testGitClient) Head(ctx context.Context, repository string) (string, error) {
	return client.run(ctx, repository, "rev-parse", "HEAD")
}

func (client testGitClient) CurrentRef(ctx context.Context, repository string) (string, error) {
	return client.run(ctx, repository, "symbolic-ref", "--short", "HEAD")
}

func (client testGitClient) Status(ctx context.Context, repository string) (string, error) {
	return client.run(ctx, repository, "status", "--porcelain")
}

func (client testGitClient) Log(ctx context.Context, repository string, limit int) (string, error) {
	return client.run(ctx, repository, "log", fmt.Sprintf("--max-count=%d", limit))
}

func (client testGitClient) CommitDetails(ctx context.Context, repository, sha string) (string, error) {
	return client.run(ctx, repository, "show", "-s", sha)
}

func (client testGitClient) CommitDiff(ctx context.Context, repository, sha string) (string, error) {
	return client.run(ctx, repository, "show", sha)
}

func (client testGitClient) ResolveCommit(ctx context.Context, repository, sha string) (string, error) {
	return client.run(ctx, repository, "rev-parse", "--verify", sha+"^{commit}")
}

func (client testGitClient) StageAll(ctx context.Context, repository string) error {
	_, err := client.run(ctx, repository, "add", "-A")
	return err
}

func (client testGitClient) CreateCheckpoint(ctx context.Context, repository, message string) error {
	_, err := client.run(
		ctx,
		repository,
		"-c",
		"user.name=remote.futrx.dev",
		"-c",
		"user.email=checkpoint@remote.futrx.dev",
		"commit",
		"-m",
		message,
	)
	return err
}

func (client testGitClient) CheckoutDetached(ctx context.Context, repository, sha string) (string, error) {
	return client.run(ctx, repository, "checkout", "--detach", sha)
}

func (testGitClient) run(ctx context.Context, repository string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\n"), nil
}

func makeGitRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatalf("make git repo: %v", err)
	}
}

func runTestGit(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
