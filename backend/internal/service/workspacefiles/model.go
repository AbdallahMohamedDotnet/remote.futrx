package workspacefiles

import (
	"io"
	"time"
)

type Node struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size,omitempty"`
	ModTime int64  `json:"modTime,omitempty"`
}

// Listing is the contents of a single directory within the workspace.
type Listing struct {
	Path      string  `json:"path"`
	Entries   []*Node `json:"entries"`
	Truncated bool    `json:"truncated"`
}

// SearchResult is a flat set of workspace entries whose name matched a query.
type SearchResult struct {
	Entries   []*Node `json:"entries"`
	Truncated bool    `json:"truncated"`
}

type File struct {
	Name    string
	ModTime time.Time
	content io.ReadSeekCloser
}

func (f *File) Content() io.ReadSeeker {
	return f.content
}

func (f *File) Close() error {
	return f.content.Close()
}

type Archive struct {
	Name     string
	root     string
	relative string
}

type Media struct {
	File        *File
	ContentType string
}
