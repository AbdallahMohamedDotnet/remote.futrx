package httphandlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
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
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

const ideBaseURL = "https://code.remote.futrx.dev/"

type ChatHandler struct {
	chats    *servicechat.Service
	projects *serviceproject.Service
	auth     *serviceauth.Service
}

func NewChatHandler(
	chats *servicechat.Service,
	projects *serviceproject.Service,
	auth *serviceauth.Service,
) *ChatHandler {
	return &ChatHandler{chats: chats, projects: projects, auth: auth}
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
		case "read":
			h.handleMarkRead(w, r, id)
		case "ide-open":
			h.handleIDEOpen(w, r, meta)
		case "media-open":
			h.handleMediaOpen(w, r, meta)
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

func (h *ChatHandler) handleIDEOpen(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filePath, workspaceRoot, err := resolveIDEOpenPath(r.URL.Query().Get("path"), meta.Cwd)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}

	openIDEFileSoon(filePath)
	http.Redirect(w, r, ideFolderURL(workspaceRoot), http.StatusFound)
}

func (h *ChatHandler) handleMediaOpen(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filePath, _, err := resolveIDEOpenPath(r.URL.Query().Get("path"), meta.Cwd)
	if err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}
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

func resolveIDEOpenPath(rawPath, cwd string) (string, string, error) {
	workspaceRoot := workspaceRootFromPath(cwd)
	if workspaceRoot == "" {
		return "", "", errors.New("chat workspace is unavailable")
	}

	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", "", errors.New("path is required")
	}

	cleaned := filepath.Clean(rawPath)
	if isContainerWorkspacePath(cleaned) {
		rel := strings.TrimPrefix(cleaned, "/workspace")
		filePath := filepath.Join(workspaceRoot, strings.TrimPrefix(rel, "/"))
		if !pathInside(filePath, workspaceRoot) {
			return "", "", errors.New("path escapes workspace")
		}
		return filePath, workspaceRoot, nil
	}

	if !filepath.IsAbs(cleaned) {
		return "", "", errors.New("path must be absolute")
	}
	if !pathInside(cleaned, workspaceRoot) {
		return "", "", errors.New("path is outside this chat workspace")
	}
	return cleaned, workspaceRoot, nil
}

func openIDEFileSoon(filePath string) {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		deadline := time.Now().Add(20 * time.Second)
		var lastErr error
		for time.Now().Before(deadline) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			output, err := exec.CommandContext(ctx, "code-server", "--reuse-window", filePath).CombinedOutput()
			cancel()
			if err == nil {
				return
			}
			lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
			time.Sleep(750 * time.Millisecond)
		}
		log.Printf("ide open %s: %v", filePath, lastErr)
	}()
}

func ideFolderURL(workspaceRoot string) string {
	u, err := url.Parse(ideBaseURL)
	if err != nil {
		return ideBaseURL
	}
	q := u.Query()
	q.Set("folder", workspaceRoot)
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
