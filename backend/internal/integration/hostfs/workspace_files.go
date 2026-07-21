package hostfs

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
)

type WorkspaceFileStore struct{}

func NewWorkspaceFileStore() *WorkspaceFileStore {
	return &WorkspaceFileStore{}
}

func (s *WorkspaceFileStore) DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (s *WorkspaceFileStore) ReadTree(root string, maxNodes, maxDepth int) ([]*serviceworkspacefiles.Node, bool) {
	count := 0
	return readFileTree(root, "", 0, &count, maxNodes, maxDepth)
}

func (s *WorkspaceFileStore) OpenFile(path string) (io.ReadSeekCloser, string, time.Time, error) {
	file, err := os.Open(path)
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

func (s *WorkspaceFileStore) WriteArchive(root string, destination io.Writer) error {
	archive := zip.NewWriter(destination)
	defer archive.Close()
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		target, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer source.Close()
		_, _ = io.Copy(target, source)
		return nil
	})
}

func readFileTree(
	absoluteDirectory string,
	relativeDirectory string,
	depth int,
	count *int,
	maxNodes int,
	maxDepth int,
) ([]*serviceworkspacefiles.Node, bool) {
	entries, err := os.ReadDir(absoluteDirectory)
	if err != nil {
		return nil, false
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	nodes := make([]*serviceworkspacefiles.Node, 0, len(entries))
	truncated := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if *count >= maxNodes {
			truncated = true
			break
		}
		*count++

		childRelative := path.Join(relativeDirectory, entry.Name())
		node := &serviceworkspacefiles.Node{Name: entry.Name(), Path: childRelative, IsDir: entry.IsDir()}
		if entry.IsDir() {
			if depth+1 >= maxDepth {
				truncated = true
			} else {
				children, childTruncated := readFileTree(
					filepath.Join(absoluteDirectory, entry.Name()),
					childRelative,
					depth+1,
					count,
					maxNodes,
					maxDepth,
				)
				node.Children = children
				truncated = truncated || childTruncated
			}
		} else if info, err := entry.Info(); err == nil {
			node.Size = info.Size()
			node.ModTime = info.ModTime().UnixMilli()
		}
		nodes = append(nodes, node)
	}
	return nodes, truncated
}
