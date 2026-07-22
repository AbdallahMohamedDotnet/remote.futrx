package hostfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspacePreparerDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, ".browser-gui", "profile")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}

	dangling := filepath.Join(profile, "SingletonCookie")
	if err := os.Symlink("missing-cookie-target", dangling); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join(root, "workspace-loop")
	if err := os.Symlink(".", loop); err != nil {
		t.Fatal(err)
	}

	preparer := NewWorkspacePreparer(os.Getuid(), os.Getgid())
	if err := preparer.Prepare(root); err != nil {
		t.Fatalf("Prepare() with workspace symlinks: %v", err)
	}

	for _, path := range []string{dangling, loop} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("Lstat(%q): %v", path, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%q stopped being a symlink", path)
		}
	}
}
