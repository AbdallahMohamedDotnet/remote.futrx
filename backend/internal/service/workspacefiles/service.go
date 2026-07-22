package workspacefiles

import (
	"context"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/shared/workspacepath"
)

const (
	// maxDirEntries bounds a single directory listing so a folder with a huge
	// number of children can't produce an unbounded response. Browsing is lazy
	// (one level per request), so this is per-directory, not per-tree.
	maxDirEntries = 10000
	// maxSearchResults bounds a name search across the whole workspace.
	maxSearchResults = 300
	// minSearchQuery is the shortest query we will walk the tree for.
	minSearchQuery = 2
)

var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrFileNotFound     = errors.New("file not found")
	ErrFolderNotFound   = errors.New("folder not found")
	ErrUnsupportedMedia = errors.New("file type cannot be opened in browser")
)

// Store is the filesystem boundary. Every method takes the workspace root plus a
// path relative to it; the implementation is responsible for keeping access
// inside the root (including resolving symlinks that might escape it).
type Store interface {
	DirectoryExists(root, relative string) bool
	ListDir(root, relative string, maxEntries int) ([]*Node, bool, error)
	OpenFile(root, relative string) (io.ReadSeekCloser, string, time.Time, error)
	WriteArchive(ctx context.Context, root, relative string, destination io.Writer) error
	Search(root, query string, limit int) ([]*Node, bool, error)
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

// List returns the entries directly under relativePath within the workspace.
// An empty relativePath lists the workspace root.
func (s *Service) List(cwd, relativePath string) (Listing, error) {
	root := workspacepath.Root(cwd)
	if root == "" {
		return Listing{}, ErrInvalidPath
	}
	rel := cleanRelative(relativePath)
	entries, truncated, err := s.store.ListDir(root, rel, maxDirEntries)
	if err != nil {
		return Listing{}, ErrFolderNotFound
	}
	if entries == nil {
		entries = []*Node{}
	}
	return Listing{Path: rel, Entries: entries, Truncated: truncated}, nil
}

func (s *Service) OpenFile(cwd, relativePath string) (*File, error) {
	root := workspacepath.Root(cwd)
	if root == "" {
		return nil, ErrInvalidPath
	}
	rel := cleanRelative(relativePath)
	if rel == "" {
		return nil, ErrInvalidPath
	}
	content, _, modTime, err := s.store.OpenFile(root, rel)
	if err != nil {
		return nil, ErrFileNotFound
	}
	return &File{Name: path.Base(rel), ModTime: modTime, content: content}, nil
}

func (s *Service) OpenMedia(cwd, rawPath string) (Media, error) {
	target, err := workspacepath.ResolveFile(rawPath, cwd)
	if err != nil {
		return Media{}, err
	}
	contentType, supported := supportedMediaType(target.FilePath)
	if !supported {
		return Media{}, ErrUnsupportedMedia
	}
	relative, err := filepath.Rel(target.WorkspaceRoot, target.FilePath)
	if err != nil {
		return Media{}, ErrFileNotFound
	}
	content, _, modTime, err := s.store.OpenFile(target.WorkspaceRoot, filepath.ToSlash(relative))
	if err != nil {
		return Media{}, ErrFileNotFound
	}
	return Media{
		File: &File{
			Name:    filepath.Base(target.FilePath),
			ModTime: modTime,
			content: content,
		},
		ContentType: contentType,
	}, nil
}

func (s *Service) PrepareArchive(cwd, relativePath string) (Archive, error) {
	root := workspacepath.Root(cwd)
	if root == "" {
		return Archive{}, ErrInvalidPath
	}
	rel := cleanRelative(relativePath)
	if !s.store.DirectoryExists(root, rel) {
		return Archive{}, ErrFolderNotFound
	}
	name := path.Base(rel)
	if rel == "" || name == "." || name == "/" {
		name = "workspace"
	}
	return Archive{Name: name + ".zip", root: root, relative: rel}, nil
}

func (s *Service) WriteArchive(ctx context.Context, archive Archive, destination io.Writer) error {
	return s.store.WriteArchive(ctx, archive.root, archive.relative, destination)
}

func (s *Service) Search(cwd, query string) (SearchResult, error) {
	root := workspacepath.Root(cwd)
	if root == "" {
		return SearchResult{}, ErrInvalidPath
	}
	trimmed := strings.TrimSpace(query)
	if len([]rune(trimmed)) < minSearchQuery {
		return SearchResult{Entries: []*Node{}}, nil
	}
	entries, truncated, err := s.store.Search(root, trimmed, maxSearchResults)
	if err != nil {
		return SearchResult{}, ErrFolderNotFound
	}
	if entries == nil {
		entries = []*Node{}
	}
	return SearchResult{Entries: entries, Truncated: truncated}, nil
}

// cleanRelative normalises a caller-supplied path into a forward-slash path
// relative to the workspace root. Traversal ("..") is collapsed here; the store
// enforces the real containment guarantee (including symlinks).
func cleanRelative(relativePath string) string {
	relativePath = strings.TrimSpace(filepath.ToSlash(relativePath))
	if relativePath == "" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(relativePath, "/"))
	return strings.TrimPrefix(cleaned, "/")
}

func supportedMediaType(p string) (string, bool) {
	contentType, ok := mediaTypes[strings.ToLower(filepath.Ext(p))]
	return contentType, ok
}

var mediaTypes = map[string]string{
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
