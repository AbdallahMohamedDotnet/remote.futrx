package workspacefiles

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/shared/workspacepath"
)

const (
	maxTreeNodes = 5000
	maxTreeDepth = 24
)

var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrFileNotFound     = errors.New("file not found")
	ErrFolderNotFound   = errors.New("folder not found")
	ErrUnsupportedMedia = errors.New("file type cannot be opened in browser")
)

var directories = []string{".uploads", ".media"}

type Store interface {
	DirectoryExists(path string) bool
	ReadTree(root string, maxNodes, maxDepth int) ([]*Node, bool)
	OpenFile(path string) (io.ReadSeekCloser, string, time.Time, error)
	WriteArchive(root string, destination io.Writer) error
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(cwd string) Listing {
	trees := make([]*Tree, 0, len(directories))
	truncated := false
	for _, directory := range directories {
		tree := &Tree{Dir: directory, Children: []*Node{}}
		if root, ok := s.resolveRoot(cwd, directory); ok && s.store.DirectoryExists(root) {
			tree.Exists = true
			children, treeTruncated := s.store.ReadTree(root, maxTreeNodes, maxTreeDepth)
			tree.Children = children
			truncated = truncated || treeTruncated
		}
		trees = append(trees, tree)
	}
	return Listing{Trees: trees, Truncated: truncated}
}

func (s *Service) OpenFile(cwd, directory, relativePath string) (*File, error) {
	path, ok := s.resolvePath(cwd, directory, relativePath)
	if !ok {
		return nil, ErrInvalidPath
	}
	content, _, modTime, err := s.store.OpenFile(path)
	if err != nil {
		return nil, ErrFileNotFound
	}
	return &File{Name: filepath.Base(path), ModTime: modTime, content: content}, nil
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
	content, _, modTime, err := s.store.OpenFile(target.FilePath)
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

func (s *Service) PrepareArchive(cwd, directory, relativePath string) (Archive, error) {
	relativePath = strings.TrimSpace(relativePath)
	var (
		path string
		ok   bool
	)
	if relativePath == "" {
		path, ok = s.resolveRoot(cwd, directory)
	} else {
		path, ok = s.resolvePath(cwd, directory, relativePath)
	}
	if !ok {
		return Archive{}, ErrInvalidPath
	}
	if !s.store.DirectoryExists(path) {
		return Archive{}, ErrFolderNotFound
	}
	name := filepath.Base(path)
	if relativePath == "" {
		name = strings.TrimPrefix(directory, ".")
	}
	if name == "" || name == "." {
		name = "files"
	}
	return Archive{Name: name + ".zip", path: path}, nil
}

func (s *Service) WriteArchive(archive Archive, destination io.Writer) error {
	return s.store.WriteArchive(archive.path, destination)
}

func (s *Service) resolveRoot(cwd, directory string) (string, bool) {
	if !allowedDirectory(directory) {
		return "", false
	}
	workspace := workspacepath.Root(cwd)
	if workspace == "" {
		return "", false
	}
	return filepath.Join(workspace, directory), true
}

func (s *Service) resolvePath(cwd, directory, relativePath string) (string, bool) {
	root, ok := s.resolveRoot(cwd, directory)
	if !ok || strings.TrimSpace(relativePath) == "" {
		return "", false
	}
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if !workspacepath.Contains(target, root) {
		return "", false
	}
	return target, true
}

func allowedDirectory(directory string) bool {
	for _, allowed := range directories {
		if directory == allowed {
			return true
		}
	}
	return false
}

func supportedMediaType(path string) (string, bool) {
	contentType, ok := mediaTypes[strings.ToLower(filepath.Ext(path))]
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
