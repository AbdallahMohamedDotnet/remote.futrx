package httphandlers

import (
	"archive/zip"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

// File manager: browse and download files under the workspace-root .uploads
// (chat uploads) and .media (agent-generated media) directories. Access is
// confined to those two whitelisted roots; the chat resource router already
// gates these handlers behind project membership.

const (
	fileTreeMaxNodes = 5000
	fileTreeMaxDepth = 24
)

var fileManagerDirs = []string{".uploads", ".media"}

func isFileManagerDir(dir string) bool {
	for _, d := range fileManagerDirs {
		if dir == d {
			return true
		}
	}
	return false
}

// fileNode is one entry in a directory tree. Path is relative to its whitelisted
// root, with forward slashes, and is what download URLs reference.
type fileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Size     int64       `json:"size,omitempty"`
	ModTime  int64       `json:"modTime,omitempty"`
	Children []*fileNode `json:"children,omitempty"`
}

type fileTree struct {
	Dir      string      `json:"dir"`
	Exists   bool        `json:"exists"`
	Children []*fileNode `json:"children"`
}

// resolveFileManagerRoot returns the absolute path of a whitelisted dir for the
// chat, or ("", false) if the dir isn't whitelisted or the workspace is gone.
func (h *ChatHandler) resolveFileManagerRoot(meta servicechat.Meta, dir string) (string, bool) {
	if !isFileManagerDir(dir) {
		return "", false
	}
	workspace := workspaceRootFromPath(meta.Cwd)
	if workspace == "" {
		return "", false
	}
	return filepath.Join(workspace, dir), true
}

// resolveFileManagerPath safely joins a relative path under a whitelisted dir,
// rejecting anything that escapes it.
func (h *ChatHandler) resolveFileManagerPath(meta servicechat.Meta, dir, rel string) (string, bool) {
	root, ok := h.resolveFileManagerRoot(meta, dir)
	if !ok || strings.TrimSpace(rel) == "" {
		return "", false
	}
	target := filepath.Join(root, filepath.FromSlash(rel))
	if !pathInside(target, root) {
		return "", false
	}
	return target, true
}

// handleFilesList returns the full file tree of each whitelisted dir.
func (h *ChatHandler) handleFilesList(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	trees := make([]*fileTree, 0, len(fileManagerDirs))
	truncated := false
	for _, dir := range fileManagerDirs {
		tree := &fileTree{Dir: dir, Children: []*fileNode{}}
		if root, ok := h.resolveFileManagerRoot(meta, dir); ok {
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				tree.Exists = true
				count := 0
				children, tr := buildFileTree(root, "", 0, &count)
				tree.Children = children
				truncated = truncated || tr
			}
		}
		trees = append(trees, tree)
	}

	httptransport.SendJSON(w, http.StatusOK, map[string]any{
		"trees":     trees,
		"truncated": truncated,
	})
}

// buildFileTree reads one directory level and recurses, ordered folders-first
// then case-insensitive name. It stops descending past fileTreeMaxDepth and
// stops adding nodes once the shared counter reaches fileTreeMaxNodes, signaling
// truncation so the UI can flag the listing as partial.
func buildFileTree(absDir, rel string, depth int, count *int) ([]*fileNode, bool) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, false
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	nodes := make([]*fileNode, 0, len(entries))
	truncated := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // hide dotfiles, including the self-ignoring .gitignore
		}
		if *count >= fileTreeMaxNodes {
			truncated = true
			break
		}
		*count++

		childRel := path.Join(rel, e.Name())
		node := &fileNode{Name: e.Name(), Path: childRel, IsDir: e.IsDir()}
		if e.IsDir() {
			if depth+1 >= fileTreeMaxDepth {
				truncated = true
			} else {
				children, tr := buildFileTree(filepath.Join(absDir, e.Name()), childRel, depth+1, count)
				node.Children = children
				truncated = truncated || tr
			}
		} else if info, ierr := e.Info(); ierr == nil {
			node.Size = info.Size()
			node.ModTime = info.ModTime().UnixMilli()
		}
		nodes = append(nodes, node)
	}
	return nodes, truncated
}

// handleFilesDownload streams a single (possibly nested) file as an attachment.
func (h *ChatHandler) handleFilesDownload(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	target, ok := h.resolveFileManagerPath(meta, r.URL.Query().Get("dir"), r.URL.Query().Get("path"))
	if !ok {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		httptransport.SendErr(w, http.StatusNotFound, "file not found")
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(target)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, target)
}

// handleFilesDownloadFolder streams a whitelisted dir, or a subfolder of it, as
// a zip archive. An empty path means the whole whitelisted root.
func (h *ChatHandler) handleFilesDownloadFolder(w http.ResponseWriter, r *http.Request, meta servicechat.Meta) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	dir := r.URL.Query().Get("dir")
	rel := strings.TrimSpace(r.URL.Query().Get("path"))

	var target string
	var ok bool
	if rel == "" {
		target, ok = h.resolveFileManagerRoot(meta, dir)
	} else {
		target, ok = h.resolveFileManagerPath(meta, dir, rel)
	}
	if !ok {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		httptransport.SendErr(w, http.StatusNotFound, "folder not found")
		return
	}

	zipName := filepath.Base(target)
	if rel == "" {
		zipName = strings.TrimPrefix(dir, ".") // ".uploads" -> "uploads"
	}
	if zipName == "" || zipName == "." {
		zipName = "files"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": zipName + ".zip"}))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(target, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		archiveRel, rerr := filepath.Rel(target, p)
		if rerr != nil {
			return nil
		}
		zf, cerr := zw.Create(filepath.ToSlash(archiveRel))
		if cerr != nil {
			return nil
		}
		src, oerr := os.Open(p)
		if oerr != nil {
			return nil
		}
		defer src.Close()
		_, _ = io.Copy(zf, src)
		return nil
	})
}
