package httptransport

// Resumable file uploads via the tus protocol (https://tus.io).
//
// Flow:
//   1. Client POSTs to /api/uploads with Upload-Length + Upload-Metadata
//      (chatId + filename, base64 per the tus spec) — server returns a
//      Location pointing to the new upload's URL.
//   2. Client PATCHes that URL with the next chunk and an Upload-Offset
//      header; the server appends and returns the new offset. Repeated until
//      the byte total reaches Upload-Length.
//   3. Server emits a CompleteUploads event; our consumer moves the
//      finished blob from the chunk temp dir into the chat's workspace cwd
//      with the proper ownership + mode.
//
// Chunks live under <tmpRoot>/chunks (default /var/lib/remote/uploads/tmp);
// finished files are moved to the chat-resolved cwd. We intentionally do
// NOT use /tmp — it's tmpfs on this host and would RAM-back multi-GB uploads.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tusd "github.com/tus/tusd/v2/pkg/handler"
	"github.com/tus/tusd/v2/pkg/filestore"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

const (
	// containerRootHostUID mirrors the LXD unprivileged idmap so files
	// land in the workspace owned by container-root.
	containerRootHostUID = 1000000

	// 10 GiB max — adjust via UPLOAD_MAX_BYTES if needed.
	defaultUploadMaxBytes int64 = 10 << 30
)

// ChatUploadResolver resolves a chat id to the host cwd that uploads should
// land in. Satisfied by *servicechat.Service.
type ChatUploadResolver interface {
	UploadTarget(ctx context.Context, id servicechat.ID) (string, error)
}

// UploadHandler exposes the tus protocol at /api/uploads and consumes
// completion events asynchronously.
type UploadHandler struct {
	tus     *tusd.UnroutedHandler
	chats   ChatUploadResolver
	tmpRoot string
}

func NewUploadHandler(chats ChatUploadResolver, dataDir string) (*UploadHandler, error) {
	tmpRoot := filepath.Join(dataDir, "uploads", "tmp")
	if err := os.MkdirAll(tmpRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create upload tmp: %w", err)
	}

	store := filestore.New(tmpRoot)
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)

	maxBytes := defaultUploadMaxBytes
	if v := os.Getenv("UPLOAD_MAX_BYTES"); v != "" {
		var parsed int64
		if _, err := fmt.Sscan(v, &parsed); err == nil && parsed > 0 {
			maxBytes = parsed
		}
	}

	cfg := tusd.Config{
		BasePath:                "/api/uploads/",
		StoreComposer:           composer,
		MaxSize:                 maxBytes,
		NotifyCompleteUploads:   true,
		NotifyTerminatedUploads: true,
		RespectForwardedHeaders: true,
	}
	h, err := tusd.NewUnroutedHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("tusd handler: %w", err)
	}

	uh := &UploadHandler{tus: h, chats: chats, tmpRoot: tmpRoot}
	go uh.drainCompletions(h.CompleteUploads)
	go uh.drainTerminations(h.TerminatedUploads)
	return uh, nil
}

func (u *UploadHandler) RegisterRoutes(mux *http.ServeMux) {
	// tusd v2's extractIDFromPath does NOT strip the BasePath — it just
	// trims slashes off whatever r.URL.Path it sees. Without StripPrefix
	// the upload ID ends up as "api/uploads/<id>" and every PATCH 404s.
	handler := http.StripPrefix("/api/uploads", u.tus.Middleware(http.HandlerFunc(u.dispatch)))
	mux.Handle("/api/uploads", handler)
	mux.Handle("/api/uploads/", handler)
}

// dispatch routes one tus request to the right method handler. The tus
// middleware already filtered OPTIONS / version checks / CORS before we
// got here.
func (u *UploadHandler) dispatch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		u.tus.PostFile(w, r)
	case http.MethodHead:
		u.tus.HeadFile(w, r)
	case http.MethodPatch:
		u.tus.PatchFile(w, r)
	case http.MethodGet:
		u.tus.GetFile(w, r)
	case http.MethodDelete:
		u.tus.DelFile(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (u *UploadHandler) drainCompletions(events <-chan tusd.HookEvent) {
	for ev := range events {
		if err := u.finalize(ev); err != nil {
			log.Printf("upload[%s] finalize: %v", ev.Upload.ID, err)
		}
	}
}

func (u *UploadHandler) drainTerminations(events <-chan tusd.HookEvent) {
	for ev := range events {
		// tusd's filestore already removes the .info + .bin when an upload
		// is DELETEd. We just log for visibility.
		log.Printf("upload[%s] terminated", ev.Upload.ID)
	}
}

// finalize moves the chunked file out of the tus tmpRoot and into the chat's
// host cwd. Chowns to the container-mapped uid + restrictive mode.
func (u *UploadHandler) finalize(ev tusd.HookEvent) error {
	meta := ev.Upload.MetaData
	chatID := strings.TrimSpace(meta["chatId"])
	if chatID == "" {
		return errors.New("upload metadata missing chatId")
	}
	filename := strings.TrimSpace(meta["filename"])
	if filename == "" {
		return errors.New("upload metadata missing filename")
	}
	base := filepath.Base(filename)
	if base == "" || base == "." || base == ".." || strings.ContainsAny(base, "/\\") {
		return fmt.Errorf("invalid filename: %q", filename)
	}

	cwd, err := u.chats.UploadTarget(context.Background(), servicechat.ID(chatID))
	if err != nil {
		return fmt.Errorf("resolve chat %s cwd: %w", chatID, err)
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return fmt.Errorf("chat cwd not a directory: %s", cwd)
	}

	src := filepath.Join(u.tmpRoot, ev.Upload.ID)
	dest := filepath.Join(cwd, base)

	// Refuse to overwrite. The browser side disables the upload button when
	// the same name is already in the workspace, but races are possible.
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("destination exists: %s", dest)
	}

	// Rename if the temp file lives on the same filesystem as the workspace,
	// otherwise fall back to streaming copy (tmp is /var/lib/remote/uploads,
	// workspace is /var/lib/remote/projects — usually same mount).
	if err := os.Rename(src, dest); err != nil {
		if err := copyAndRemove(src, dest); err != nil {
			return fmt.Errorf("move upload: %w", err)
		}
	}

	if err := os.Chmod(dest, 0o644); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Chown(dest, containerRootHostUID, containerRootHostUID); err != nil {
		// Non-fatal — the file is there, just owned by host root. The
		// container's root may still read it depending on idmap mode.
		log.Printf("upload[%s] chown %s: %v", ev.Upload.ID, dest, err)
	}

	// tusd leaves a .info sidecar next to the file; clean it up.
	_ = os.Remove(src + ".info")

	log.Printf("upload[%s] %d B -> %s", ev.Upload.ID, ev.Upload.Size, dest)
	return nil
}

func copyAndRemove(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// metaJSON is reserved for the frontend in case we want to surface upload
// metadata back to a poll endpoint. Unused for now.
var _ = json.Marshal
