package workspacepath

import (
	"path/filepath"
	"strings"
)

func Root(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return ""
	}
	if path == "/workspace" {
		return path
	}
	marker := string(filepath.Separator) + "workspace"
	if strings.HasSuffix(path, marker) {
		return path
	}
	needle := marker + string(filepath.Separator)
	if index := strings.Index(path, needle); index >= 0 {
		return path[:index+len(marker)]
	}
	return path
}

func Contains(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
