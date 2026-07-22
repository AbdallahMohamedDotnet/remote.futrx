// Package hostfs provides host-filesystem adapters used by application
// services.
package hostfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WorkspacePreparer creates and remaps a host workspace for an unprivileged
// container bind mount.
type WorkspacePreparer struct {
	uid int
	gid int
}

func NewWorkspacePreparer(uid, gid int) *WorkspacePreparer {
	return &WorkspacePreparer{uid: uid, gid: gid}
}

func (p *WorkspacePreparer) Prepare(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create workspace dir: %w", err)
	}
	if err := chownRecursively(path, p.uid, p.gid); err != nil {
		return fmt.Errorf("chown workspace: %w", err)
	}
	return nil
}

func chownRecursively(path string, uid, gid int) error {
	root := filepath.Clean(path)
	return filepath.WalkDir(root, func(current string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Browser profiles contain short-lived lock entries. A concurrent
			// browser shutdown may remove one between reading the directory and
			// visiting it; that must not prevent the workspace from launching.
			if current != root && errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			return walkErr
		}

		// Lchown changes a symlink itself instead of following it. Chromium's
		// Singleton* locks intentionally point at ephemeral files and sockets,
		// which are commonly dangling after a container recycle.
		if err := os.Lchown(current, uid, gid); err != nil {
			if current != root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	})
}
