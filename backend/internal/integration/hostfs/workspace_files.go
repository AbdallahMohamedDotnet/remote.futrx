package hostfs

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
	"github.com/futrx-com/remote.futrx.com/internal/shared/workspacepath"
)

// maxSearchVisits bounds how many entries a workspace-wide name search will walk
// before giving up, so searching a repo with a huge node_modules stays bounded.
const maxSearchVisits = 200000

var errOutsideWorkspace = errors.New("path escapes workspace")

type WorkspaceFileStore struct{}

func NewWorkspaceFileStore() *WorkspaceFileStore {
	return &WorkspaceFileStore{}
}

func (s *WorkspaceFileStore) DirectoryExists(root, relative string) bool {
	resolved, err := secureResolve(root, relative)
	if err != nil {
		return false
	}
	info, err := os.Stat(resolved)
	return err == nil && info.IsDir()
}

func (s *WorkspaceFileStore) ListDir(root, relative string, maxEntries int) ([]*serviceworkspacefiles.Node, bool, error) {
	resolved, err := secureResolve(root, relative)
	if err != nil {
		return nil, false, err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, false, err
	}
	sortEntries(entries)

	nodes := make([]*serviceworkspacefiles.Node, 0, len(entries))
	truncated := false
	for _, entry := range entries {
		if len(nodes) >= maxEntries {
			truncated = true
			break
		}
		node, ok := nodeFor(root, relative, entry)
		if !ok {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, truncated, nil
}

func (s *WorkspaceFileStore) OpenFile(root, relative string) (io.ReadSeekCloser, string, time.Time, error) {
	resolved, err := secureResolve(root, relative)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		_ = file.Close()
		return nil, "", time.Time{}, os.ErrNotExist
	}
	return file, info.Name(), info.ModTime(), nil
}

func (s *WorkspaceFileStore) WriteArchive(root, relative string, destination io.Writer) error {
	base, err := secureResolve(root, relative)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return err
	}

	archive := zip.NewWriter(destination)
	defer archive.Close()

	// WalkDir does not descend into symlinked directories, so directory-symlink
	// loops are impossible. Symlinked files are included only if they still
	// resolve inside the workspace.
	return filepath.WalkDir(base, func(walkPath string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		openPath := walkPath
		if entry.Type()&os.ModeSymlink != 0 {
			target, resolveErr := filepath.EvalSymlinks(walkPath)
			if resolveErr != nil || !workspacepath.Contains(target, resolvedRoot) {
				return nil
			}
			info, statErr := os.Stat(target)
			if statErr != nil || info.IsDir() {
				return nil
			}
			openPath = target
		}
		relative, relErr := filepath.Rel(base, walkPath)
		if relErr != nil {
			return nil
		}
		writer, createErr := archive.Create(filepath.ToSlash(relative))
		if createErr != nil {
			return nil
		}
		source, openErr := os.Open(openPath)
		if openErr != nil {
			return nil
		}
		defer source.Close()
		_, _ = io.Copy(writer, source)
		return nil
	})
}

func (s *WorkspaceFileStore) Search(root, query string, limit int) ([]*serviceworkspacefiles.Node, bool, error) {
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return nil, false, err
	}
	needle := strings.ToLower(query)

	var results []*serviceworkspacefiles.Node
	truncated := false
	visits := 0
	walkErr := filepath.WalkDir(resolvedRoot, func(walkPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if walkPath == resolvedRoot {
			return nil
		}
		visits++
		if visits > maxSearchVisits {
			truncated = true
			return filepath.SkipAll
		}
		if !strings.Contains(strings.ToLower(entry.Name()), needle) {
			return nil
		}
		relative, relErr := filepath.Rel(resolvedRoot, walkPath)
		if relErr != nil {
			return nil
		}
		node := &serviceworkspacefiles.Node{
			Name:  entry.Name(),
			Path:  filepath.ToSlash(relative),
			IsDir: entry.IsDir(),
		}
		if !entry.IsDir() {
			if info, infoErr := entry.Info(); infoErr == nil {
				node.Size = info.Size()
				node.ModTime = info.ModTime().UnixMilli()
			}
		}
		results = append(results, node)
		if len(results) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return nil, false, walkErr
	}
	return results, truncated, nil
}

// nodeFor builds a listing node for a directory entry. Symlinks are resolved and
// dropped if they escape the workspace; the returned Path is always the entry's
// workspace-relative path so the client can request it back safely.
func nodeFor(root, parentRelative string, entry os.DirEntry) (*serviceworkspacefiles.Node, bool) {
	name := entry.Name()
	childRelative := path.Join(parentRelative, name)

	isDir := entry.IsDir()
	var size, modTime int64

	if entry.Type()&os.ModeSymlink != 0 {
		resolved, err := secureResolve(root, childRelative)
		if err != nil {
			return nil, false
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, false
		}
		isDir = info.IsDir()
		if !isDir {
			size = info.Size()
		}
		modTime = info.ModTime().UnixMilli()
	} else if info, err := entry.Info(); err == nil {
		if !isDir {
			size = info.Size()
		}
		modTime = info.ModTime().UnixMilli()
	}

	node := &serviceworkspacefiles.Node{Name: name, Path: childRelative, IsDir: isDir, ModTime: modTime}
	if !isDir {
		node.Size = size
	}
	return node, true
}

func sortEntries(entries []os.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
}

// secureResolve joins relative under root, collapses traversal, then resolves
// symlinks and verifies the final real path is still inside the real workspace
// root. It is the single gate that keeps file access from escaping the
// workspace even when the container has planted symlinks into it.
func secureResolve(root, relative string) (string, error) {
	root = filepath.Clean(root)
	rel := filepath.Join(string(filepath.Separator), filepath.FromSlash(relative))
	target := filepath.Join(root, rel)
	if !workspacepath.Contains(target, root) {
		return "", errOutsideWorkspace
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if !workspacepath.Contains(resolved, resolvedRoot) {
		return "", errOutsideWorkspace
	}
	return resolved, nil
}
