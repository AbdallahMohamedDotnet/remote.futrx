package http

import (
	"io/fs"
	"mime"
	"net/http"

	model "futrx.local/catalog/applications/file-manager/backend/container/internal/model/files/application"
	"futrx.local/catalog/applications/file-manager/backend/container/internal/port/files/inbound"
)

type handler struct {
	files inbound.Files
}

func New(files inbound.Files, assets fs.FS) http.Handler {
	handler := &handler{files: files}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("file-manager ready")) })
	mux.HandleFunc("GET /api/list", handler.list)
	mux.HandleFunc("GET /api/search", handler.search)
	mux.HandleFunc("GET /api/download", handler.download)
	mux.HandleFunc("GET /api/archive", handler.archive)
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	return pageHeaders(mux)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	result, err := h.files.List(r.URL.Query().Get("path"))
	if err != nil {
		sendError(w, err)
		return
	}
	sendJSON(w, listing{Path: result.Path, Entries: presentNodes(result.Entries), Truncated: result.Truncated})
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	result, err := h.files.Search(r.URL.Query().Get("q"))
	if err != nil {
		sendError(w, err)
		return
	}
	sendJSON(w, searchResult{Entries: presentNodes(result.Entries), Truncated: result.Truncated})
}

func (h *handler) download(w http.ResponseWriter, r *http.Request) {
	file, err := h.files.OpenFile(r.URL.Query().Get("path"))
	if err != nil {
		sendError(w, err)
		return
	}
	defer file.Close()
	serveDownload(w, r, file)
}

func (h *handler) archive(w http.ResponseWriter, r *http.Request) {
	file, err := h.files.DownloadArchive(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		if r.Context().Err() == nil {
			sendError(w, err)
		}
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/zip")
	serveDownload(w, r, file)
}

func serveDownload(w http.ResponseWriter, r *http.Request, file *model.File) {
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	http.ServeContent(w, r, file.Name, file.ModTime, file.Content())
}
