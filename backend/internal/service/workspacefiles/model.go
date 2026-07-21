package workspacefiles

import (
	"io"
	"time"
)

type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	IsDir    bool    `json:"isDir"`
	Size     int64   `json:"size,omitempty"`
	ModTime  int64   `json:"modTime,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

type Tree struct {
	Dir      string  `json:"dir"`
	Exists   bool    `json:"exists"`
	Children []*Node `json:"children"`
}

type Listing struct {
	Trees     []*Tree `json:"trees"`
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
	Name string
	path string
}
