package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	serviceworkspacefiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/workspacefiles"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

const (
	ideBaseURL                  = "https://code.remote.futrx.dev/"
	codeServerSessionSocketPath = "/root/.local/share/code-server/code-server-ipc.sock"
)

type ChatHandler struct {
	chats    *servicechat.Service
	projects *serviceproject.Service
	auth     *serviceauth.Service
	files    *serviceworkspacefiles.Service
}

func NewChatHandler(
	chats *servicechat.Service,
	projects *serviceproject.Service,
	auth *serviceauth.Service,
	files *serviceworkspacefiles.Service,
) *ChatHandler {
	return &ChatHandler{chats: chats, projects: projects, auth: auth, files: files}
}

func (h *ChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/chats", h.HandleCollection)
	mux.HandleFunc("/api/chats/", h.HandleResource)
}

func (h *ChatHandler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	email, isAdmin, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		metas, err := h.chats.List(r.Context())
		if err != nil {
			sendChatError(w, err)
			return
		}
		filtered := h.filterChats(r.Context(), metas, email, isAdmin)
		if filtered == nil {
			filtered = []servicechat.Meta{}
		}
		httptransport.SendJSON(w, http.StatusOK, filtered)

	case http.MethodPost:
		var in servicechat.CreateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil && err != io.EOF {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if in.ProjectID != "" {
			ok, err := h.canAccessProject(r.Context(), serviceproject.ID(in.ProjectID), email, isAdmin)
			if err != nil {
				sendChatError(w, err)
				return
			}
			if !ok {
				httptransport.SendErr(w, http.StatusForbidden, "not a member of this project")
				return
			}
		}
		meta, err := h.chats.Create(r.Context(), in)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusCreated, meta)

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ChatHandler) HandleResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.SplitN(rest, "/", 2)
	id := servicechat.ID(parts[0])

	email, isAdmin, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Resolve the chat once so we can check project membership before any
	// action (read or write). 404 if the chat doesn't exist; 403 if it
	// belongs to a project the caller can't see.
	meta, err := h.chats.Get(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	if meta.ProjectID != "" {
		ok, err := h.canAccessProject(r.Context(), serviceproject.ID(meta.ProjectID), email, isAdmin)
		if err != nil {
			sendChatError(w, err)
			return
		}
		if !ok {
			httptransport.SendErr(w, http.StatusForbidden, "not a member of this chat's project")
			return
		}
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "events":
			h.handleEvents(w, r, id)
		case "rewind":
			h.handleRewind(w, r, id)
		case "fork":
			h.handleFork(w, r, id)
		case "read":
			h.handleMarkRead(w, r, id)
		case "unread":
			h.handleMarkUnread(w, r, id)
		case "ide-open":
			h.handleIDEOpen(w, r, meta)
		case "media-open":
			h.handleMediaOpen(w, r, meta)
		case "files":
			h.handleFilesList(w, r, meta)
		case "files/download":
			h.handleFilesDownload(w, r, meta)
		case "files/download-folder":
			h.handleFilesDownloadFolder(w, r, meta)
		case "history/repos":
			h.handleHistoryRepos(w, r, meta)
		case "history/commits":
			h.handleHistoryCommits(w, r, meta)
		case "history/diff":
			h.handleHistoryDiff(w, r, meta)
		case "history/checkout":
			h.handleHistoryCheckout(w, r, meta)
		default:
			httptransport.SendErr(w, http.StatusNotFound, "not found")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		httptransport.SendJSON(w, http.StatusOK, meta)

	case http.MethodPatch:
		var in servicechat.UpdateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		updated, err := h.chats.Update(r.Context(), id, in)
		if err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := h.chats.Delete(r.Context(), id); err != nil {
			sendChatError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ChatHandler) handleEvents(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	page, err := h.chats.EventPage(r.Context(), id, servicechat.EventPageQuery{
		Limit:     intQuery(r, "limit", 200),
		BeforeSeq: int64Query(r, "before", 0),
	})
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, page)
}

func (h *ChatHandler) handleRewind(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		BeforeT int64 `json:"beforeT"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := h.chats.Rewind(r.Context(), id, body.BeforeT); err != nil {
		sendChatError(w, err)
		return
	}
	page, err := h.chats.EventPage(r.Context(), id, servicechat.EventPageQuery{Limit: 200})
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, page)
}

func (h *ChatHandler) handleFork(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	forked, err := h.chats.Fork(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusCreated, forked)
}

func (h *ChatHandler) handleMarkRead(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	meta, err := h.chats.MarkRead(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, meta)
}

func (h *ChatHandler) handleMarkUnread(w http.ResponseWriter, r *http.Request, id servicechat.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	meta, err := h.chats.MarkUnread(r.Context(), id)
	if err != nil {
		sendChatError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, meta)
}

func (h *ChatHandler) handleIDEOpen(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	target, err := resolveIDEOpenPath(r.URL.Query().Get("path"), meta.Cwd)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	target.WorkspaceName = h.ideWorkspaceName(r.Context(), meta, target.WorkspaceRoot)

	// Per-container IDE: redirect into the project's own code-server (runs as the
	// container user, so git ownership stays consistent). The folder+file query
	// makes code-server open the workspace and the file directly -- no host IDE
	// IPC, which would otherwise open the file in the wrong place.
	http.Redirect(w, r, ideOpenRedirectURL(target), http.StatusFound)
}

func (h *ChatHandler) handleMediaOpen(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	target, err := resolveIDEOpenPath(r.URL.Query().Get("path"), meta.Cwd)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
	filePath := target.FilePath
	if !isBrowserMediaFile(filePath) {
		httptransport.SendErr(w, http.StatusUnsupportedMediaType, "file type cannot be opened in browser")
		return
	}
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		httptransport.SendErr(w, http.StatusNotFound, "file not found")
		return
	}

	if contentType := mediaContentType(filePath); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filepath.Base(filePath)}))
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data: blob:; media-src 'self' data: blob:; style-src 'unsafe-inline'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filePath)
}

type ideOpenTarget struct {
	FilePath      string
	WorkspaceRoot string
	WorkspaceName string
	Line          int
	Column        int
}

func resolveIDEOpenPath(rawPath, cwd string) (ideOpenTarget, error) {
	workspaceRoot := workspaceRootFromPath(cwd)
	if workspaceRoot == "" {
		return ideOpenTarget{}, errors.New("chat workspace is unavailable")
	}

	rawPath, line, column := parsePathLineReference(rawPath)
	if rawPath == "" {
		return ideOpenTarget{}, errors.New("path is required")
	}

	cleaned := filepath.Clean(rawPath)
	if isContainerWorkspacePath(cleaned) {
		rel := strings.TrimPrefix(cleaned, "/workspace")
		filePath := filepath.Join(workspaceRoot, strings.TrimPrefix(rel, "/"))
		if !pathInside(filePath, workspaceRoot) {
			return ideOpenTarget{}, errors.New("path escapes workspace")
		}
		return ideOpenTarget{FilePath: filePath, WorkspaceRoot: workspaceRoot, Line: line, Column: column}, nil
	}

	if !filepath.IsAbs(cleaned) {
		return ideOpenTarget{}, errors.New("path must be absolute")
	}
	if !pathInside(cleaned, workspaceRoot) {
		return ideOpenTarget{}, errors.New("path is outside this chat workspace")
	}
	return ideOpenTarget{FilePath: cleaned, WorkspaceRoot: workspaceRoot, Line: line, Column: column}, nil
}

func parsePathLineReference(rawPath string) (string, int, int) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", 0, 0
	}

	if hashIndex := strings.Index(rawPath, "#"); hashIndex >= 0 {
		fragment := rawPath[hashIndex+1:]
		rawPath = rawPath[:hashIndex]
		if queryIndex := strings.Index(rawPath, "?"); queryIndex >= 0 {
			rawPath = rawPath[:queryIndex]
		}
		if line, column := parseLineFragment(fragment); line > 0 {
			return strings.TrimSpace(rawPath), line, column
		}
	}
	if queryIndex := strings.Index(rawPath, "?"); queryIndex >= 0 {
		rawPath = rawPath[:queryIndex]
	}

	rawPath, line, column := splitPathLineSuffix(rawPath)
	return strings.TrimSpace(rawPath), line, column
}

func parseLineFragment(fragment string) (int, int) {
	fragment = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(fragment), "l"))
	if fragment == "" {
		return 0, 0
	}
	parts := strings.SplitN(fragment, ":", 2)
	line, err := strconv.Atoi(parts[0])
	if err != nil || line <= 0 {
		return 0, 0
	}
	if len(parts) == 2 {
		column, err := strconv.Atoi(parts[1])
		if err == nil && column > 0 {
			return line, column
		}
	}
	return line, 0
}

func splitPathLineSuffix(rawPath string) (string, int, int) {
	rawPath = strings.TrimSpace(rawPath)
	lastColon := strings.LastIndex(rawPath, ":")
	if lastColon < 0 {
		return rawPath, 0, 0
	}
	lastPart := rawPath[lastColon+1:]
	lastNumber, err := strconv.Atoi(lastPart)
	if err != nil || lastNumber <= 0 {
		return rawPath, 0, 0
	}

	beforeLast := rawPath[:lastColon]
	secondColon := strings.LastIndex(beforeLast, ":")
	if secondColon >= 0 {
		maybeLine, err := strconv.Atoi(beforeLast[secondColon+1:])
		if err == nil && maybeLine > 0 {
			return beforeLast[:secondColon], maybeLine, lastNumber
		}
	}
	return beforeLast, lastNumber, 0
}

type codeServerSessionResponse struct {
	SocketPath string `json:"socketPath"`
}

type codeServerOpenRequest struct {
	Type             string   `json:"type"`
	FolderURIs       []string `json:"folderURIs"`
	FileURIs         []string `json:"fileURIs"`
	GotoLineMode     bool     `json:"gotoLineMode"`
	ForceReuseWindow bool     `json:"forceReuseWindow"`
	ForceNewWindow   bool     `json:"forceNewWindow"`
}

func openIDEFileInExistingWindow(ctx context.Context, target ideOpenTarget) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	socketPath, err := codeServerMatchingSocket(ctx, target.FilePath)
	if err != nil || socketPath == "" {
		return false, err
	}
	if err := postCodeServerOpen(ctx, socketPath, target); err != nil {
		return false, err
	}
	return true, nil
}

func (h *ChatHandler) ideWorkspaceName(ctx context.Context, meta servicechat.Meta, workspaceRoot string) string {
	if h.projects != nil && meta.ProjectID != "" {
		projectMeta, err := h.projects.Get(ctx, serviceproject.ID(meta.ProjectID))
		if err == nil && strings.TrimSpace(projectMeta.Name) != "" {
			return strings.TrimSpace(projectMeta.Name)
		}
	}
	return workspaceNameFromRoot(workspaceRoot)
}

func workspaceNameFromRoot(workspaceRoot string) string {
	parent := filepath.Base(filepath.Dir(filepath.Clean(workspaceRoot)))
	name := strings.TrimSpace(strings.ReplaceAll(parent, "-", " "))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "Workspace"
	}
	return name
}

func codeServerMatchingSocket(ctx context.Context, filePath string) (string, error) {
	var out codeServerSessionResponse
	path := "/session?filePath=" + url.QueryEscape(filePath)
	if err := codeServerUnixJSON(ctx, codeServerSessionSocketPath, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return out.SocketPath, nil
}

func postCodeServerOpen(ctx context.Context, socketPath string, target ideOpenTarget) error {
	payload := codeServerOpenRequest{
		Type:             "open",
		FolderURIs:       []string{},
		FileURIs:         []string{ideOpenFileURI(target)},
		GotoLineMode:     target.Line > 0,
		ForceReuseWindow: true,
		ForceNewWindow:   false,
	}
	return codeServerUnixJSON(ctx, socketPath, http.MethodPost, "/", payload, nil)
}

func codeServerUnixJSON(ctx context.Context, socketPath, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, method, "http://code-server"+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("code-server ipc %s %s: %s", method, path, strings.TrimSpace(string(message)))
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	return nil
}

func ideOpenFileURI(target ideOpenTarget) string {
	return (&url.URL{Scheme: "file", Path: ideOpenCommandTarget(target)}).String()
}

func sendIDEOpenExistingResponse(w http.ResponseWriter, target ideOpenTarget) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Opened in IDE</title><script>window.close()</script><style>body{font:14px system-ui,sans-serif;margin:24px;color:#20242a}code{background:#f3f4f6;padding:2px 4px;border-radius:4px}</style></head><body>Opened <code>%s</code> in the existing IDE window.</body></html>`, htmlEscaper.Replace(filepath.Base(target.FilePath)))
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&#39;",
)

func openIDEFileSoon(target ideOpenTarget) {
	// Map host paths -> project container + in-container paths. Non-project paths
	// keep host paths and reuseSlug "" (host code-server fallback).
	reuseSlug, workspaceTarget, _ := containerSlugAndPath(target.WorkspaceRoot)
	if workspaceTarget == "" {
		workspaceTarget = target.WorkspaceRoot
	}
	fileSlug, containerFile, okFile := containerSlugAndPath(target.FilePath)
	var fileTarget string
	if okFile {
		reuseSlug = fileSlug
		fileTarget = containerOpenCommandTarget(containerFile, target.Line, target.Column)
	} else {
		fileTarget = ideOpenCommandTarget(target)
	}

	go func() {
		var lastErr error

		// Give the clicked code-server tab time to load and register its workspace.
		time.Sleep(1500 * time.Millisecond)

		workspaceDeadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(workspaceDeadline) {
			if err := runCodeServerReuse(reuseSlug, workspaceTarget); err == nil {
				break
			} else {
				lastErr = err
			}
			time.Sleep(750 * time.Millisecond)
		}

		fileStart := time.Now()
		fileDeadline := fileStart.Add(20 * time.Second)
		opened := false
		for time.Now().Before(fileDeadline) {
			if err := runCodeServerReuse(reuseSlug, fileTarget); err == nil {
				opened = true
			} else {
				lastErr = err
			}

			if opened && time.Since(fileStart) >= 7*time.Second {
				return
			}
			time.Sleep(1 * time.Second)
		}
		if !opened {
			log.Printf("ide open %s: %v", fileTarget, lastErr)
		}
	}()
}

// runCodeServerReuse focuses/opens target in an already-running code-server,
// reusing its window. For a project (slug != "") it runs inside that project's
// container so the file opens in the per-container IDE; otherwise it falls back
// to the host code-server (e.g. platform-repo chats).
func runCodeServerReuse(slug, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if slug == "" {
		cmd = exec.CommandContext(ctx, "code-server", "--reuse-window", target)
	} else {
		cmd = exec.CommandContext(ctx, "lxc", "exec", slug, "--", "code-server", "--reuse-window", target)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// containerSlugAndPath maps a host path under a project workspace
// (/var/lib/remote/projects/<slug>/workspace[/rel]) to the project slug and the
// equivalent in-container path (/workspace[/rel]). ok is false for paths not
// under a project workspace (e.g. the platform's own /opt repo).
func containerSlugAndPath(hostPath string) (slug, containerPath string, ok bool) {
	const prefix = "/var/lib/remote/projects/"
	rest, found := strings.CutPrefix(filepath.Clean(hostPath), prefix)
	if !found {
		return "", "", false
	}
	slug, after, found := strings.Cut(rest, "/")
	if !found || slug == "" {
		return "", "", false
	}
	if after == "workspace" {
		return slug, "/workspace", true
	}
	rel, found := strings.CutPrefix(after, "workspace/")
	if !found {
		return "", "", false
	}
	return slug, "/workspace/" + rel, true
}

func containerOpenCommandTarget(containerFile string, line, column int) string {
	if line <= 0 {
		return containerFile
	}
	if column > 0 {
		return fmt.Sprintf("%s:%d:%d", containerFile, line, column)
	}
	return fmt.Sprintf("%s:%d", containerFile, line)
}

func ideOpenCommandTarget(target ideOpenTarget) string {
	if target.Line <= 0 {
		return target.FilePath
	}
	if target.Column > 0 {
		return fmt.Sprintf("%s:%d:%d", target.FilePath, target.Line, target.Column)
	}
	return fmt.Sprintf("%s:%d", target.FilePath, target.Line)
}

type codeWorkspaceFile struct {
	Folders  []codeWorkspaceFolder `json:"folders"`
	Settings map[string]string     `json:"settings,omitempty"`
}

type codeWorkspaceFolder struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path"`
}

func ideOpenURL(target ideOpenTarget) string {
	workspacePath, err := writeIDEWorkspaceFile(target)
	if err != nil {
		log.Printf("ide workspace file %s: %v", target.WorkspaceRoot, err)
		return ideFolderURL(target.WorkspaceRoot)
	}
	return ideWorkspaceURL(workspacePath)
}

func writeIDEWorkspaceFile(target ideOpenTarget) (string, error) {
	workspaceRoot := filepath.Clean(target.WorkspaceRoot)
	if workspaceRoot == "" || workspaceRoot == "." || !filepath.IsAbs(workspaceRoot) {
		return "", errors.New("workspace root is unavailable")
	}
	name := strings.TrimSpace(target.WorkspaceName)
	if name == "" {
		name = workspaceNameFromRoot(workspaceRoot)
	}

	dir := filepath.Join(filepath.Dir(workspaceRoot), ".futrx", "code-server")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(dir, safeWorkspaceFilename(name)+".code-workspace")
	doc := codeWorkspaceFile{
		Folders: []codeWorkspaceFolder{{Name: name, Path: workspaceRoot}},
		Settings: map[string]string{
			"window.title": "${activeEditorShort} - ${rootName}",
		},
	}
	contents, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	contents = append(contents, byte(10))
	if err := os.WriteFile(workspacePath, contents, 0o644); err != nil {
		return "", err
	}
	return workspacePath, nil
}

func safeWorkspaceFilename(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	lastSpace := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSpace = false
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastSpace = false
		case r == ' ' || r == 9 || r == 10 || r == 13:
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
		if b.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if out == "" {
		return "Workspace"
	}
	return out
}

func ideWorkspaceURL(workspacePath string) string {
	u, err := url.Parse(ideBaseURL)
	if err != nil {
		return ideBaseURL
	}
	q := u.Query()
	q.Set("workspace", workspacePath)
	u.RawQuery = q.Encode()
	return u.String()
}

func ideFolderURL(workspaceRoot string) string {
	base := ideBaseURL
	folder := workspaceRoot
	if slug, containerRoot, ok := containerSlugAndPath(workspaceRoot); ok {
		base = "https://code.remote.futrx.dev/" + slug + "/"
		folder = containerRoot
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("folder", folder)
	u.RawQuery = q.Encode()
	return u.String()
}

// ideOpenRedirectURL builds the per-container IDE URL that opens the workspace
// folder and (when known) focuses the file -- container paths under
// <slug>.code.remote.futrx.dev. Falls back to the host code-server for paths
// not under a project workspace.
func ideOpenRedirectURL(target ideOpenTarget) string {
	base := ideBaseURL
	folder := target.WorkspaceRoot
	file := target.FilePath
	if slug, containerRoot, ok := containerSlugAndPath(target.WorkspaceRoot); ok {
		base = "https://code.remote.futrx.dev/" + slug + "/"
		folder = containerRoot
		if _, containerFile, okFile := containerSlugAndPath(target.FilePath); okFile {
			file = containerFile
		}
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("folder", folder)
	if file != "" && file != folder {
		q.Set("file", file)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func isBrowserMediaFile(path string) bool {
	_, ok := browserMediaExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func mediaContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if contentType, ok := browserMediaExtensions[ext]; ok {
		return contentType
	}
	return mime.TypeByExtension(ext)
}

var browserMediaExtensions = map[string]string{
	".aac":  "audio/aac",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".flac": "audio/flac",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".m4a":  "audio/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".oga":  "audio/ogg",
	".ogg":  "audio/ogg",
	".ogv":  "video/ogg",
	".opus": "audio/opus",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
	".wav":  "audio/wav",
	".webm": "video/webm",
	".webp": "image/webp",
}

func workspaceRootFromPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return ""
	}
	if path == "/workspace" {
		return path
	}
	marker := string(filepath.Separator) + "workspace"
	if strings.HasSuffix(path, marker) {
		return path
	}
	needle := marker + string(filepath.Separator)
	if idx := strings.Index(path, needle); idx >= 0 {
		return path[:idx+len(marker)]
	}
	return path
}

func isContainerWorkspacePath(path string) bool {
	return path == "/workspace" || strings.HasPrefix(path, "/workspace/")
}

func pathInside(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// canAccessProject returns true if the caller is an admin OR a member of
// the project. Admins always pass. Members are looked up in the access list.
func (h *ChatHandler) canAccessProject(ctx context.Context, id serviceproject.ID, email string, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if h.projects == nil || email == "" {
		return false, nil
	}
	return h.projects.HasAccess(ctx, id, email)
}

// filterChats removes chats that belong to projects the caller can't see.
// Chats with no projectId are always visible (they predate per-project
// access and don't have a container behind them).
func (h *ChatHandler) filterChats(ctx context.Context, metas []servicechat.Meta, email string, isAdmin bool) []servicechat.Meta {
	if isAdmin || h.projects == nil {
		return metas
	}
	out := make([]servicechat.Meta, 0, len(metas))
	for _, m := range metas {
		if m.ProjectID == "" {
			out = append(out, m)
			continue
		}
		ok, err := h.projects.HasAccess(ctx, serviceproject.ID(m.ProjectID), email)
		if err == nil && ok {
			out = append(out, m)
		}
	}
	return out
}

func (h *ChatHandler) caller(r *http.Request) (string, bool, error) {
	if h.auth == nil {
		return "", true, nil
	}
	return callerStateFromRequest(r.Context(), r, h.auth)
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Query(r *http.Request, key string, fallback int64) int64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func sendChatError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, servicechat.ErrInvalidID),
		errors.Is(err, servicechat.ErrInvalidTmuxSession),
		errors.Is(err, servicechat.ErrInvalidRewindTimestamp):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, servicechat.ErrNotFound):
		httptransport.SendErr(w, http.StatusNotFound, "chat not found")
	case errors.Is(err, servicechat.ErrChatRunning):
		httptransport.SendErr(w, http.StatusConflict, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
