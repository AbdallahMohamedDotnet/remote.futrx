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
	root, err := os.OpenRoot(filepath.Clean(path))
	if err != nil {
		return err
	}
	walkErr := fs.WalkDir(root.FS(), ".", func(current string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Browser profiles contain short-lived lock entries. A concurrent
			// browser shutdown may remove one between reading the directory and
			// visiting it; that must not prevent the workspace from launching.
			if current != "." && errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			return walkErr
		}

		// Root.Lchown changes the final symlink itself and resolves every parent
		// relative to the opened workspace descriptor. A concurrent directory to
		// symlink swap therefore cannot redirect ownership changes outside it.
		if err := root.Lchown(current, uid, gid); err != nil {
			if current != "." && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	})
	return errors.Join(walkErr, root.Close())
}
