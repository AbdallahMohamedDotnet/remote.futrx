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

// secureWorkspace owns the workspace containment boundary for one store
// operation. Resolving the real root once keeps every path decision within the
// same trusted context and avoids scattering containment checks across file
// operations.
type secureWorkspace struct {
	root     string
	realRoot string
}

func NewWorkspaceFileStore() *WorkspaceFileStore {
	return &WorkspaceFileStore{}
}

func (s *WorkspaceFileStore) DirectoryExists(root, relative string) bool {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return false
	}
	resolved, err := workspace.resolve(relative)
	if err != nil {
		return false
	}
	info, err := os.Stat(resolved)
	return err == nil && info.IsDir()
}

func (s *WorkspaceFileStore) ListDir(root, relative string, maxEntries int) ([]*serviceworkspacefiles.Node, bool, error) {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return nil, false, err
	}
	resolved, err := workspace.resolve(relative)
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
		node, ok := workspace.nodeFor(relative, entry)
		if !ok {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, truncated, nil
}

func (s *WorkspaceFileStore) OpenFile(root, relative string) (io.ReadSeekCloser, string, time.Time, error) {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	resolved, err := workspace.resolve(relative)
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
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return err
	}
	base, err := workspace.resolve(relative)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(destination)

	// WalkDir does not descend into symlinked directories, so directory-symlink
	// loops are impossible. Symlinked files are included only if they still
	// resolve inside the workspace.
	walkErr := filepath.WalkDir(base, func(walkPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		return workspace.writeArchiveEntry(archive, base, walkPath, entry)
	})
	return errors.Join(walkErr, archive.Close())
}

func (s *WorkspaceFileStore) Search(root, query string, limit int) ([]*serviceworkspacefiles.Node, bool, error) {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return nil, false, err
	}
	needle := strings.ToLower(query)

	var results []*serviceworkspacefiles.Node
	truncated := false
	visits := 0
	walkErr := filepath.WalkDir(workspace.realRoot, func(walkPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if walkPath == workspace.realRoot {
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
		relative, relErr := filepath.Rel(workspace.realRoot, walkPath)
		if relErr != nil {
			return nil
		}
		workspaceRelative := filepath.ToSlash(relative)
		node, ok := workspace.nodeFor(path.Dir(workspaceRelative), entry)
		if !ok {
			return nil
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

func (w secureWorkspace) writeArchiveEntry(
	archive *zip.Writer,
	base string,
	walkPath string,
	entry os.DirEntry,
) error {
	openPath := walkPath
	if entry.Type()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(walkPath)
		if err != nil || !workspacepath.Contains(target, w.realRoot) {
			// Broken and escaping symlinks are intentionally omitted: neither is a
			// usable file inside the workspace boundary.
			return nil
		}
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		openPath = target
	}

	relative, err := filepath.Rel(base, walkPath)
	if err != nil {
		return err
	}
	destination, err := archive.Create(filepath.ToSlash(relative))
	if err != nil {
		return err
	}
	source, err := os.Open(openPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	return errors.Join(copyErr, source.Close())
}

// nodeFor builds a listing node for a directory entry. Symlinks are resolved and
// dropped if they escape the workspace; the returned Path is always the entry's
// workspace-relative path so the client can request it back safely.
func (w secureWorkspace) nodeFor(parentRelative string, entry os.DirEntry) (*serviceworkspacefiles.Node, bool) {
	name := entry.Name()
	childRelative := path.Join(parentRelative, name)

	isDir := entry.IsDir()
	var size, modTime int64

	if entry.Type()&os.ModeSymlink != 0 {
		resolved, err := w.resolve(childRelative)
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

func newSecureWorkspace(root string) (secureWorkspace, error) {
	cleanRoot := filepath.Clean(root)
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		return secureWorkspace{}, err
	}
	return secureWorkspace{root: cleanRoot, realRoot: realRoot}, nil
}

// resolve joins relative under root, collapses traversal, then resolves
// symlinks and verifies the final real path is still inside the real workspace
// root. It is the single gate that keeps file access from escaping the
// workspace even when the container has planted symlinks into it.
func (w secureWorkspace) resolve(relative string) (string, error) {
	rel := filepath.Join(string(filepath.Separator), filepath.FromSlash(relative))
	target := filepath.Join(w.root, rel)
	if !workspacepath.Contains(target, w.root) {
		return "", errOutsideWorkspace
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if !workspacepath.Contains(resolved, w.realRoot) {
		return "", errOutsideWorkspace
	}
	return resolved, nil
}
