// Package hostfs provides host-filesystem adapters used by application
// services.
package hostfs

import (
	"fmt"
	"os"
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
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := chownRecursively(path+"/"+entry.Name(), uid, gid); err != nil {
			return err
		}
	}
	return nil
}
