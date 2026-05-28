package upload

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
)

// HandleMultipart reads files from a multipart form (field name "files") and
// writes each to cwd, refusing to overwrite.
func HandleMultipart(cwd string, w http.ResponseWriter, r *http.Request) {
	const maxTotal = 200 << 20 // 200 MiB per request
	r.Body = http.MaxBytesReader(w, r.Body, maxTotal)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpserver.SendErr(w, 400, "upload too large or malformed: "+err.Error())
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		httpserver.SendErr(w, 500, "cwd is not a directory: "+cwd)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		httpserver.SendErr(w, 400, "no files in request (field name: files)")
		return
	}

	type result struct {
		Name  string `json:"name"`
		Path  string `json:"path,omitempty"`
		Size  int64  `json:"size,omitempty"`
		Error string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(files))

	for _, fh := range files {
		res := result{Name: fh.Filename}
		base := filepath.Base(fh.Filename)
		if base == "" || base == "." || base == ".." ||
			strings.ContainsAny(base, "/\\") {
			res.Error = "invalid filename"
			results = append(results, res)
			continue
		}
		dest := filepath.Join(cwd, base)
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				res.Error = "file already exists at " + dest
			} else {
				res.Error = err.Error()
			}
			results = append(results, res)
			continue
		}
		src, err := fh.Open()
		if err != nil {
			out.Close()
			os.Remove(dest)
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		n, err := io.Copy(out, src)
		src.Close()
		_ = out.Close()
		if err != nil {
			os.Remove(dest)
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		res.Path = dest
		res.Size = n
		results = append(results, res)
	}

	httpserver.SendJSON(w, 200, map[string]any{"cwd": cwd, "results": results})
}
