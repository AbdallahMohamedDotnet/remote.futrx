package hostfs

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
	"github.com/futrx-com/remote.futrx.com/internal/shared/workspacepath"
)

const (
	// maxSearchVisits bounds how many entries a workspace-wide name search will
	// walk before giving up, so searching a huge node_modules stays bounded.
	maxSearchVisits = 200000
	// Archive limits apply to source data before compression. This prevents a
	// sparse or highly compressible workspace from consuming unbounded CPU and
	// read I/O while remaining below the compressed spool-file limit.
	maxArchiveSourceBytes int64 = 1 << 30
	maxArchiveEntries           = 200000
)

type archiveLimits struct {
	maxSourceBytes int64
	maxEntries     int
}

var defaultArchiveLimits = archiveLimits{
	maxSourceBytes: maxArchiveSourceBytes,
	maxEntries:     maxArchiveEntries,
}

var (
	errOutsideWorkspace = errors.New("path escapes workspace")
	errNotRegularFile   = errors.New("not a regular file")
)

type WorkspaceFileStore struct{}

// secureWorkspace owns the workspace containment boundary for one store
// operation. The os.Root handle makes the final filesystem operation relative
// to an opened directory, so a concurrent symlink swap cannot redirect it
// outside the workspace after path validation.
type secureWorkspace struct {
	root     *os.Root
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
	defer workspace.close()
	resolved, err := workspace.resolve(relative)
	if err != nil {
		return false
	}
	info, err := workspace.root.Stat(resolved)
	return err == nil && info.IsDir()
}

func (s *WorkspaceFileStore) ListDir(root, relative string, maxEntries int) ([]*serviceworkspacefiles.Node, bool, error) {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return nil, false, err
	}
	defer workspace.close()
	resolved, err := workspace.resolve(relative)
	if err != nil {
		return nil, false, err
	}
	directory, err := workspace.root.Open(resolved)
	if err != nil {
		return nil, false, err
	}
	entries, err := directory.ReadDir(-1)
	_ = directory.Close()
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
	defer workspace.close()
	resolved, err := workspace.resolve(relative)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	file, info, err := workspace.openRegular(resolved)
	if err != nil {
		if errors.Is(err, errNotRegularFile) {
			return nil, "", time.Time{}, os.ErrNotExist
		}
		return nil, "", time.Time{}, err
	}
	return file, info.Name(), info.ModTime(), nil
}

func (s *WorkspaceFileStore) WriteArchive(ctx context.Context, root, relative string, destination io.Writer) error {
	return s.writeArchive(ctx, root, relative, destination, defaultArchiveLimits)
}

func (s *WorkspaceFileStore) writeArchive(
	ctx context.Context,
	root string,
	relative string,
	destination io.Writer,
	limits archiveLimits,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return err
	}
	defer workspace.close()
	base, err := workspace.resolve(relative)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(destination)
	budget := archiveBudget{
		remainingBytes:   limits.maxSourceBytes,
		remainingEntries: limits.maxEntries,
	}

	// WalkDir does not descend into symlinked directories, so directory-symlink
	// loops are impossible. Symlinked files are included only if they still
	// resolve inside the workspace.
	walkErr := fs.WalkDir(workspace.root.FS(), base, func(walkPath string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		return workspace.writeArchiveEntry(ctx, archive, &budget, base, walkPath, entry)
	})
	closeErr := archive.Close()
	return errors.Join(walkErr, closeErr, ctx.Err())
}

func (s *WorkspaceFileStore) Search(root, query string, limit int) ([]*serviceworkspacefiles.Node, bool, error) {
	workspace, err := newSecureWorkspace(root)
	if err != nil {
		return nil, false, err
	}
	defer workspace.close()
	needle := strings.ToLower(query)

	var results []*serviceworkspacefiles.Node
	truncated := false
	visits := 0
	walkErr := fs.WalkDir(workspace.root.FS(), ".", func(walkPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if walkPath == "." {
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
		node, ok := workspace.nodeFor(path.Dir(walkPath), entry)
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
	ctx context.Context,
	archive *zip.Writer,
	budget *archiveBudget,
	base string,
	walkPath string,
	entry fs.DirEntry,
) error {
	openPath := walkPath
	if entry.Type()&os.ModeSymlink != 0 {
		target, err := w.resolve(walkPath)
		if err != nil {
			// Broken and escaping symlinks are intentionally omitted: neither is a
			// usable file inside the workspace boundary.
			return nil
		}
		openPath = target
	}

	source, info, err := w.openRegular(openPath)
	if err != nil {
		if errors.Is(err, errNotRegularFile) {
			return nil
		}
		return err
	}
	if err := budget.acceptEntry(info.Size()); err != nil {
		return errors.Join(err, source.Close())
	}
	relative, err := filepath.Rel(base, walkPath)
	if err != nil {
		return errors.Join(err, source.Close())
	}
	destination, err := archive.Create(filepath.ToSlash(relative))
	if err != nil {
		return errors.Join(err, source.Close())
	}
	copyErr := budget.copy(ctx, destination, source)
	return errors.Join(copyErr, source.Close())
}

type archiveBudget struct {
	remainingBytes   int64
	remainingEntries int
}

func (b *archiveBudget) acceptEntry(size int64) error {
	if b.remainingEntries <= 0 || size > b.remainingBytes {
		return serviceworkspacefiles.ErrArchiveTooLarge
	}
	b.remainingEntries--
	return nil
}

func (b *archiveBudget) copy(ctx context.Context, destination io.Writer, source io.Reader) error {
	reader := contextReader{ctx: ctx, reader: source}
	written, copyErr := io.Copy(destination, io.LimitReader(reader, b.remainingBytes+1))
	if written > b.remainingBytes {
		b.remainingBytes = 0
		return errors.Join(copyErr, serviceworkspacefiles.ErrArchiveTooLarge)
	}
	b.remainingBytes -= written
	return copyErr
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

// nodeFor builds a listing node for a directory entry. Symlinks are resolved and
// dropped if they escape the workspace; the returned Path is always the entry's
// workspace-relative path so the client can request it back safely.
func (w secureWorkspace) nodeFor(parentRelative string, entry fs.DirEntry) (*serviceworkspacefiles.Node, bool) {
	name := entry.Name()
	childRelative := path.Join(parentRelative, name)

	isDir := entry.IsDir()
	var size, modTime int64

	if entry.Type()&os.ModeSymlink != 0 {
		resolved, err := w.resolve(childRelative)
		if err != nil {
			return nil, false
		}
		info, err := w.root.Stat(resolved)
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

func sortEntries(entries []fs.DirEntry) {
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
	rootHandle, err := os.OpenRoot(realRoot)
	if err != nil {
		return secureWorkspace{}, err
	}
	return secureWorkspace{root: rootHandle, realRoot: realRoot}, nil
}

func (w secureWorkspace) close() {
	_ = w.root.Close()
}

// openRegular checks before opening to avoid touching known special files, then
// opens nonblocking and verifies the resulting descriptor. The second check
// closes the race where a regular path is swapped for a FIFO or device between
// Stat and OpenFile.
func (w secureWorkspace) openRegular(name string) (*os.File, os.FileInfo, error) {
	info, err := w.root.Stat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errNotRegularFile
	}

	file, err := w.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, errNotRegularFile
	}
	return file, openedInfo, nil
}

// resolve joins relative under root, collapses traversal, then resolves
// symlinks and translates the result to an os.Root-relative name. The final
// open/stat still goes through os.Root, which enforces containment if the path
// changes after this resolution. Resolving here preserves support for absolute
// symlinks whose targets remain inside the workspace; os.Root intentionally
// rejects absolute symlinks when asked to follow them directly.
func (w secureWorkspace) resolve(relative string) (string, error) {
	rel := filepath.Join(string(filepath.Separator), filepath.FromSlash(relative))
	target := filepath.Join(w.realRoot, rel)
	if !workspacepath.Contains(target, w.realRoot) {
		return "", errOutsideWorkspace
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if !workspacepath.Contains(resolved, w.realRoot) {
		return "", errOutsideWorkspace
	}
	resolvedRelative, err := filepath.Rel(w.realRoot, resolved)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(resolvedRelative), nil
}
