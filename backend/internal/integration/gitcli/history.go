package gitcli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type HistoryClient struct{}

const commitFormat = "%H%x1f%h%x1f%an%x1f%ae%x1f%at%x1f%s"

func NewHistoryClient() *HistoryClient {
	return &HistoryClient{}
}

func (c *HistoryClient) DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (c *HistoryClient) IsRepository(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func (c *HistoryClient) DiscoverRepositories(root string, maxDepth int, skippedDirectories []string) []string {
	root = filepath.Clean(root)
	skipped := make(map[string]bool, len(skippedDirectories))
	for _, directory := range skippedDirectories {
		skipped[directory] = true
	}

	seen := map[string]bool{}
	repositories := []string{}
	add := func(path string) {
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			repositories = append(repositories, path)
		}
	}
	if c.IsRepository(root) {
		add(root)
	}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root {
			if skipped[entry.Name()] || relativeDepth(root, path) > maxDepth {
				return filepath.SkipDir
			}
		}
		if c.IsRepository(path) {
			add(path)
		}
		return nil
	})
	return repositories
}

func (c *HistoryClient) Head(ctx context.Context, repositoryPath string) (string, error) {
	return c.run(ctx, repositoryPath, 5*time.Second, "rev-parse", "HEAD")
}

func (c *HistoryClient) CurrentRef(ctx context.Context, repositoryPath string) (string, error) {
	return c.run(ctx, repositoryPath, 5*time.Second, "symbolic-ref", "--short", "HEAD")
}

func (c *HistoryClient) Status(ctx context.Context, repositoryPath string) (string, error) {
	return c.run(ctx, repositoryPath, 5*time.Second, "status", "--porcelain")
}

func (c *HistoryClient) Log(ctx context.Context, repositoryPath string, limit int) (string, error) {
	return c.run(
		ctx,
		repositoryPath,
		10*time.Second,
		"log",
		"--all",
		"--date-order",
		fmt.Sprintf("--max-count=%d", limit),
		"--pretty=format:"+commitFormat,
	)
}

func (c *HistoryClient) CommitDetails(ctx context.Context, repositoryPath, sha string) (string, error) {
	return c.run(ctx, repositoryPath, 5*time.Second, "show", "-s", "--pretty=format:"+commitFormat, sha)
}

func (c *HistoryClient) CommitDiff(ctx context.Context, repositoryPath, sha string) (string, error) {
	return c.run(
		ctx,
		repositoryPath,
		15*time.Second,
		"show",
		"--format=",
		"--patch",
		"--no-ext-diff",
		"--unified=3",
		"--find-renames",
		"--no-color",
		sha,
		"--",
	)
}

func (c *HistoryClient) ResolveCommit(ctx context.Context, repositoryPath, sha string) (string, error) {
	return c.run(ctx, repositoryPath, 5*time.Second, "rev-parse", "--verify", sha+"^{commit}")
}

func (c *HistoryClient) StageAll(ctx context.Context, repositoryPath string) error {
	_, err := c.run(ctx, repositoryPath, 20*time.Second, "add", "-A")
	return err
}

func (c *HistoryClient) CreateCheckpoint(ctx context.Context, repositoryPath, message string) error {
	_, err := c.run(
		ctx,
		repositoryPath,
		20*time.Second,
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

func (c *HistoryClient) CheckoutDetached(ctx context.Context, repositoryPath, sha string) (string, error) {
	return c.run(ctx, repositoryPath, 20*time.Second, "checkout", "--detach", sha)
}

func (c *HistoryClient) run(
	ctx context.Context,
	repositoryPath string,
	timeout time.Duration,
	args ...string,
) (string, error) {
	commandContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	fullArgs := append([]string{"-c", "safe.directory=" + repositoryPath, "-C", repositoryPath}, args...)
	command := exec.CommandContext(commandContext, "git", fullArgs...)
	output, err := command.CombinedOutput()
	result := strings.TrimRight(string(output), "\n")
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), result)
	}
	return result, nil
}

func relativeDepth(root, path string) int {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." {
		return 0
	}
	return strings.Count(filepath.ToSlash(relative), "/") + 1
}
