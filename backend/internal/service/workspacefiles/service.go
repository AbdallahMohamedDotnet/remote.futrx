package workspacefiles

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/shared/workspacepath"
)

const (
	maxTreeNodes = 5000
	maxTreeDepth = 24
)

var (
	ErrInvalidPath    = errors.New("invalid path")
	ErrFileNotFound   = errors.New("file not found")
	ErrFolderNotFound = errors.New("folder not found")
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
	content, name, modTime, err := s.store.OpenFile(path)
	if err != nil {
		return nil, ErrFileNotFound
	}
	return &File{Name: name, ModTime: modTime, content: content}, nil
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
