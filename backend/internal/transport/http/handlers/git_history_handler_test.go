package httphandlers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGitHistoryRepoPath(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	makeGitRepo(t, workspaceRoot)
	makeGitRepo(t, filepath.Join(workspaceRoot, "packages", "app"))
	nestedRepo := filepath.Join(workspaceRoot, "packages", "app")
	tests := []struct {
		name    string
		rawRepo string
		want    string
	}{
		{name: "root", rawRepo: ".", want: workspaceRoot},
		{name: "relative", rawRepo: "packages/app", want: nestedRepo},
		{name: "container", rawRepo: "/workspace/packages/app", want: nestedRepo},
		{name: "host", rawRepo: nestedRepo, want: nestedRepo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGitHistoryRepoPath(tt.rawRepo, workspaceRoot)
			if err != nil {
				t.Fatalf("resolveGitHistoryRepoPath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveGitHistoryRepoPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveGitHistoryRepoPathRejectsUnsafePaths(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	makeGitRepo(t, workspaceRoot)
	tests := []string{"../other", "/workspace/../etc", filepath.Join(filepath.Dir(workspaceRoot), "other"), "/etc"}

	for _, rawRepo := range tests {
		t.Run(rawRepo, func(t *testing.T) {
			if got, err := resolveGitHistoryRepoPath(rawRepo, workspaceRoot); err == nil {
				t.Fatalf("resolveGitHistoryRepoPath() = %q, want error", got)
			}
		})
	}
}

func TestParseGitHistoryCommitLine(t *testing.T) {
	line := "0123456789abcdef\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f1712345678\x1fAdd history drawer"
	commit, err := parseGitHistoryCommitLine(line, "0123456789abcdef")
	if err != nil {
		t.Fatalf("parseGitHistoryCommitLine() error = %v", err)
	}
	if !commit.IsHead || commit.ShortSha != "0123456" || commit.Subject != "Add history drawer" || commit.AuthorDate != 1712345678 {
		t.Fatalf("parseGitHistoryCommitLine() = %+v", commit)
	}
}

func TestParseGitHistoryDirtyFiles(t *testing.T) {
	status := " M app.tsx\n?? new-file.txt\nR  old.txt -> new.txt\n"
	files := parseGitHistoryDirtyFiles(status)
	want := []string{"M app.tsx", "?? new-file.txt", "R  old.txt -> new.txt"}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("parseGitHistoryDirtyFiles() = %#v, want %#v", files, want)
	}
}

func TestSanitizeGitHistoryCheckpointMessage(t *testing.T) {
	got := sanitizeGitHistoryCheckpointMessage("  checkpoint\n before\t switch  ")
	if got != "checkpoint before switch" {
		t.Fatalf("sanitizeGitHistoryCheckpointMessage() = %q", got)
	}
}

func TestCheckpointGitHistoryChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	if err := os.WriteFile(filepath.Join(repo, "app.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "app.txt")
	runTestGit(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(repo, "app.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sha, err := checkpointGitHistoryChanges(context.Background(), repo, "  checkpoint\n before switch  ")
	if err != nil {
		t.Fatalf("checkpointGitHistoryChanges() error = %v", err)
	}
	if sha == "" {
		t.Fatal("checkpointGitHistoryChanges() returned empty sha")
	}
	status, dirtyFiles, err := gitHistoryRepoStatus(context.Background(), repo)
	if err != nil {
		t.Fatalf("gitHistoryRepoStatus() error = %v", err)
	}
	if status != "" || len(dirtyFiles) != 0 {
		t.Fatalf("status = %q, dirtyFiles = %#v; want clean", status, dirtyFiles)
	}
	msg := runTestGit(t, repo, "log", "-1", "--pretty=%s")
	if strings.TrimSpace(msg) != "checkpoint before switch" {
		t.Fatalf("commit subject = %q", msg)
	}
}

func makeGitRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatalf("make git repo: %v", err)
	}
}

func runTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
