package httphandlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

const (
	gitHistoryMaxDepth   = 6
	gitHistoryDiffMaxLen = 768 * 1024
)

type gitHistoryRepo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	RelativePath string   `json:"relativePath"`
	CurrentRef   string   `json:"currentRef"`
	CurrentSha   string   `json:"currentSha"`
	Dirty        bool     `json:"dirty"`
	DirtyFiles   []string `json:"dirtyFiles,omitempty"`
}

type gitHistoryCommit struct {
	Sha         string `json:"sha"`
	ShortSha    string `json:"shortSha"`
	Subject     string `json:"subject"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	AuthorDate  int64  `json:"authorDate"`
	IsHead      bool   `json:"isHead"`
}

type gitHistoryReposResponse struct {
	WorkspaceRoot string           `json:"workspaceRoot"`
	Repos         []gitHistoryRepo `json:"repos"`
}

type gitHistoryCommitsResponse struct {
	Repo    gitHistoryRepo     `json:"repo"`
	Commits []gitHistoryCommit `json:"commits"`
}

type gitHistoryDiffResponse struct {
	Repo      gitHistoryRepo   `json:"repo"`
	Commit    gitHistoryCommit `json:"commit"`
	Diff      string           `json:"diff"`
	Truncated bool             `json:"truncated"`
}

type gitHistoryCheckoutResponse struct {
	Repo          gitHistoryRepo `json:"repo"`
	Output        string         `json:"output,omitempty"`
	CheckpointSha string         `json:"checkpointSha,omitempty"`
}

func (h *ChatHandler) handleHistoryRepos(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	workspaceRoot, err := historyWorkspaceRoot(meta)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	repos, err := listGitHistoryRepos(r.Context(), workspaceRoot)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, gitHistoryReposResponse{WorkspaceRoot: workspaceRoot, Repos: repos})
}

func (h *ChatHandler) handleHistoryCommits(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	repoPath, repo, ok := h.historyRepoFromRequest(w, r, meta, r.URL.Query().Get("repo"))
	if !ok {
		return
	}
	limit := intQuery(r, "limit", 80)
	if limit < 1 {
		limit = 80
	}
	if limit > 200 {
		limit = 200
	}
	commits, err := listGitHistoryCommits(r.Context(), repoPath, repo.CurrentSha, limit)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, gitHistoryCommitsResponse{Repo: repo, Commits: commits})
}

func (h *ChatHandler) handleHistoryDiff(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	repoPath, repo, ok := h.historyRepoFromRequest(w, r, meta, r.URL.Query().Get("repo"))
	if !ok {
		return
	}
	commit, err := gitHistoryCommitForSha(r.Context(), repoPath, repo.CurrentSha, r.URL.Query().Get("sha"))
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	diff, truncated, err := gitHistoryCommitDiff(r.Context(), repoPath, commit.Sha)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, gitHistoryDiffResponse{Repo: repo, Commit: commit, Diff: diff, Truncated: truncated})
}

func (h *ChatHandler) handleHistoryCheckout(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Repo              string `json:"repo"`
		Sha               string `json:"sha"`
		CheckpointMessage string `json:"checkpointMessage"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	repoPath, _, ok := h.historyRepoFromRequest(w, r, meta, body.Repo)
	if !ok {
		return
	}
	sha, err := verifyGitHistoryCommit(r.Context(), repoPath, body.Sha)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_, dirtyFiles, err := gitHistoryRepoStatus(r.Context(), repoPath)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var checkpointSha string
	if len(dirtyFiles) > 0 {
		message := strings.TrimSpace(body.CheckpointMessage)
		if message == "" {
			httptransport.SendJSON(w, http.StatusConflict, map[string]any{
				"error":      "dirty working tree",
				"dirty":      true,
				"dirtyFiles": dirtyFiles,
			})
			return
		}
		checkpointSha, err = checkpointGitHistoryChanges(r.Context(), repoPath, message)
		if err != nil {
			httptransport.SendErr(w, http.StatusConflict, err.Error())
			return
		}
	}

	output, err := runGitHistory(r.Context(), repoPath, 20*time.Second, "checkout", "--detach", sha)
	if err != nil {
		httptransport.SendErr(w, http.StatusConflict, err.Error())
		return
	}
	workspaceRoot, err := historyWorkspaceRoot(meta)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	repo, err := describeGitHistoryRepo(r.Context(), workspaceRoot, repoPath)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, gitHistoryCheckoutResponse{Repo: repo, Output: output, CheckpointSha: checkpointSha})
}

func (h *ChatHandler) historyRepoFromRequest(w http.ResponseWriter, r *http.Request, meta servicechat.Meta, rawRepo string) (string, gitHistoryRepo, bool) {
	workspaceRoot, err := historyWorkspaceRoot(meta)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return "", gitHistoryRepo{}, false
	}
	repoPath, err := resolveGitHistoryRepoPath(rawRepo, workspaceRoot)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return "", gitHistoryRepo{}, false
	}
	repo, err := describeGitHistoryRepo(r.Context(), workspaceRoot, repoPath)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return "", gitHistoryRepo{}, false
	}
	return repoPath, repo, true
}

func historyWorkspaceRoot(meta servicechat.Meta) (string, error) {
	workspaceRoot := workspaceRootFromPath(meta.Cwd)
	if workspaceRoot == "" {
		return "", errors.New("chat workspace is unavailable")
	}
	info, err := os.Stat(workspaceRoot)
	if err != nil || !info.IsDir() {
		return "", errors.New("chat workspace is unavailable on this host")
	}
	return workspaceRoot, nil
}

func listGitHistoryRepos(ctx context.Context, workspaceRoot string) ([]gitHistoryRepo, error) {
	workspaceRoot = filepath.Clean(workspaceRoot)
	seen := map[string]bool{}
	repos := []gitHistoryRepo{}
	addRepo := func(path string) {
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		seen[path] = true
		repo, err := describeGitHistoryRepo(ctx, workspaceRoot, path)
		if err == nil {
			repos = append(repos, repo)
		}
	}

	if isGitHistoryRepo(workspaceRoot) {
		addRepo(workspaceRoot)
	}
	_ = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != workspaceRoot {
			if shouldSkipGitHistoryDir(d.Name()) {
				return filepath.SkipDir
			}
			if relativeDepth(workspaceRoot, path) > gitHistoryMaxDepth {
				return filepath.SkipDir
			}
		}
		if isGitHistoryRepo(path) {
			addRepo(path)
		}
		return nil
	})

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].ID == "." {
			return true
		}
		if repos[j].ID == "." {
			return false
		}
		return repos[i].RelativePath < repos[j].RelativePath
	})
	return repos, nil
}

func describeGitHistoryRepo(ctx context.Context, workspaceRoot, repoPath string) (gitHistoryRepo, error) {
	if !isGitHistoryRepo(repoPath) {
		return gitHistoryRepo{}, errors.New("not a git repository")
	}
	sha, err := runGitHistory(ctx, repoPath, 5*time.Second, "rev-parse", "HEAD")
	if err != nil {
		return gitHistoryRepo{}, err
	}
	ref, err := runGitHistory(ctx, repoPath, 5*time.Second, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(ref) == "" {
		ref = "detached"
	}
	_, dirtyFiles, _ := gitHistoryRepoStatus(ctx, repoPath)
	id, rel := gitHistoryRepoID(workspaceRoot, repoPath)
	name := filepath.Base(repoPath)
	if id == "." {
		name = filepath.Base(workspaceRoot)
	}
	return gitHistoryRepo{
		ID:           id,
		Name:         name,
		Path:         repoPath,
		RelativePath: rel,
		CurrentRef:   ref,
		CurrentSha:   strings.TrimSpace(sha),
		Dirty:        len(dirtyFiles) > 0,
		DirtyFiles:   dirtyFiles,
	}, nil
}

func resolveGitHistoryRepoPath(rawRepo, workspaceRoot string) (string, error) {
	workspaceRoot = filepath.Clean(workspaceRoot)
	rawRepo = strings.TrimSpace(rawRepo)
	if rawRepo == "" || rawRepo == "." {
		if !isGitHistoryRepo(workspaceRoot) {
			return "", errors.New("workspace root is not a git repository")
		}
		return workspaceRoot, nil
	}

	var repoPath string
	cleaned := filepath.Clean(rawRepo)
	if isContainerWorkspacePath(cleaned) {
		rel := strings.TrimPrefix(cleaned, "/workspace")
		repoPath = filepath.Join(workspaceRoot, strings.TrimPrefix(rel, "/"))
	} else if filepath.IsAbs(cleaned) {
		repoPath = cleaned
	} else {
		repoPath = filepath.Join(workspaceRoot, filepath.FromSlash(rawRepo))
	}
	repoPath = filepath.Clean(repoPath)
	if !pathInside(repoPath, workspaceRoot) {
		return "", errors.New("repository is outside this chat workspace")
	}
	if !isGitHistoryRepo(repoPath) {
		return "", errors.New("not a git repository")
	}
	return repoPath, nil
}

func listGitHistoryCommits(ctx context.Context, repoPath, currentSha string, limit int) ([]gitHistoryCommit, error) {
	format := "%H%x1f%h%x1f%an%x1f%ae%x1f%at%x1f%s"
	out, err := runGitHistory(ctx, repoPath, 10*time.Second, "log", "--all", "--date-order", "--max-count="+strconv.Itoa(limit), "--pretty=format:"+format)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return []gitHistoryCommit{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	commits := make([]gitHistoryCommit, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		commit, err := parseGitHistoryCommitLine(line, currentSha)
		if err == nil {
			commits = append(commits, commit)
		}
	}
	return commits, nil
}

func gitHistoryCommitForSha(ctx context.Context, repoPath, currentSha, rawSha string) (gitHistoryCommit, error) {
	sha, err := verifyGitHistoryCommit(ctx, repoPath, rawSha)
	if err != nil {
		return gitHistoryCommit{}, err
	}
	format := "%H%x1f%h%x1f%an%x1f%ae%x1f%at%x1f%s"
	out, err := runGitHistory(ctx, repoPath, 5*time.Second, "show", "-s", "--pretty=format:"+format, sha)
	if err != nil {
		return gitHistoryCommit{}, err
	}
	return parseGitHistoryCommitLine(out, currentSha)
}

func gitHistoryCommitDiff(ctx context.Context, repoPath, sha string) (string, bool, error) {
	out, err := runGitHistory(ctx, repoPath, 15*time.Second, "show", "--format=", "--patch", "--no-ext-diff", "--unified=3", "--find-renames", "--no-color", sha, "--")
	if err != nil {
		return "", false, err
	}
	truncated := false
	if len(out) > gitHistoryDiffMaxLen {
		out = out[:gitHistoryDiffMaxLen] + "\n\n[diff truncated]"
		truncated = true
	}
	return out, truncated, nil
}

func gitHistoryRepoStatus(ctx context.Context, repoPath string) (string, []string, error) {
	status, err := runGitHistory(ctx, repoPath, 5*time.Second, "status", "--porcelain")
	if err != nil {
		return "", nil, err
	}
	return status, parseGitHistoryDirtyFiles(status), nil
}

func checkpointGitHistoryChanges(ctx context.Context, repoPath, message string) (string, error) {
	message = sanitizeGitHistoryCheckpointMessage(message)
	if message == "" {
		return "", errors.New("checkpoint message is required")
	}
	if _, err := runGitHistory(ctx, repoPath, 20*time.Second, "add", "-A"); err != nil {
		return "", err
	}
	if _, dirtyFiles, err := gitHistoryRepoStatus(ctx, repoPath); err != nil {
		return "", err
	} else if len(dirtyFiles) == 0 {
		return "", errors.New("there are no changes to checkpoint")
	}
	if _, err := runGitHistory(ctx, repoPath, 20*time.Second, "-c", "user.name=remote.futrx.dev", "-c", "user.email=checkpoint@remote.futrx.dev", "commit", "-m", message); err != nil {
		return "", err
	}
	sha, err := runGitHistory(ctx, repoPath, 5*time.Second, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func parseGitHistoryDirtyFiles(status string) []string {
	lines := strings.Split(strings.TrimSpace(status), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files
}

func sanitizeGitHistoryCheckpointMessage(message string) string {
	message = strings.TrimSpace(message)
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 200 {
		message = strings.TrimSpace(message[:200])
	}
	return message
}

func verifyGitHistoryCommit(ctx context.Context, repoPath, rawSha string) (string, error) {
	sha := strings.TrimSpace(rawSha)
	if !validGitHistoryCommitID(sha) {
		return "", errors.New("invalid commit")
	}
	canonical, err := runGitHistory(ctx, repoPath, 5*time.Second, "rev-parse", "--verify", sha+"^{commit}")
	if err != nil {
		return "", errors.New("commit not found")
	}
	return strings.TrimSpace(canonical), nil
}

func parseGitHistoryCommitLine(line, currentSha string) (gitHistoryCommit, error) {
	parts := strings.SplitN(line, "\x1f", 6)
	if len(parts) != 6 {
		return gitHistoryCommit{}, errors.New("invalid commit data")
	}
	authorDate, _ := strconv.ParseInt(parts[4], 10, 64)
	sha := strings.TrimSpace(parts[0])
	return gitHistoryCommit{
		Sha:         sha,
		ShortSha:    strings.TrimSpace(parts[1]),
		Subject:     strings.TrimSpace(parts[5]),
		AuthorName:  strings.TrimSpace(parts[2]),
		AuthorEmail: strings.TrimSpace(parts[3]),
		AuthorDate:  authorDate,
		IsHead:      currentSha != "" && sha == currentSha,
	}, nil
}

func runGitHistory(ctx context.Context, repoPath string, timeout time.Duration, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	fullArgs := append([]string{"-c", "safe.directory=" + repoPath, "-C", repoPath}, args...)
	cmd := exec.CommandContext(cmdCtx, "git", fullArgs...)
	output, err := cmd.CombinedOutput()
	out := strings.TrimRight(string(output), "\n")
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), out)
	}
	return out, nil
}

func gitHistoryRepoID(workspaceRoot, repoPath string) (string, string) {
	rel, err := filepath.Rel(workspaceRoot, repoPath)
	if err != nil || rel == "." {
		return ".", "."
	}
	rel = filepath.ToSlash(rel)
	return rel, rel
}

func isGitHistoryRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func shouldSkipGitHistoryDir(name string) bool {
	switch name {
	case ".git", ".agents", ".browser", ".cache", ".claude", ".codex", ".media", ".vscode", "node_modules", ".next", "dist", "build", "out", "coverage", "vendor", "tmp", "__pycache__":
		return true
	default:
		return false
	}
}

func relativeDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func validGitHistoryCommitID(sha string) bool {
	if len(sha) < 4 || len(sha) > 64 {
		return false
	}
	for _, r := range sha {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}
