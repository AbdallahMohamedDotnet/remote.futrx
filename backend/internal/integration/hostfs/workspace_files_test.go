package hostfs

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// setupWorkspace builds a workspace tree plus an out-of-workspace secret and
// returns the workspace root.
func setupWorkspace(t *testing.T) (root, secret string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "workspace")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret = filepath.Join(base, "outside-secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, secret
}

func TestListDirShowsDotfiles(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()

	nodes, _, err := store.ListDir(root, "", 100)
	if err != nil {
		t.Fatalf("ListDir root: %v", err)
	}
	names := map[string]bool{}
	for _, n := range nodes {
		names[n.Name] = true
	}
	if !names[".env"] {
		t.Fatalf("expected dotfile .env to be listed, got %v", names)
	}
	if !names["src"] {
		t.Fatalf("expected src dir to be listed, got %v", names)
	}
}

func TestOpenFileRejectsTraversal(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()

	for _, rel := range []string{"../outside-secret.txt", "src/../../outside-secret.txt", "/etc/hosts"} {
		if _, _, _, err := store.OpenFile(root, rel); err == nil {
			t.Fatalf("OpenFile(%q) succeeded, want error", rel)
		}
	}
}

func TestSymlinkEscapeIsBlocked(t *testing.T) {
	root, secret := setupWorkspace(t)
	store := NewWorkspaceFileStore()

	// Simulate untrusted container code planting a symlink to a host file.
	link := filepath.Join(root, "escape")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Downloading the escaping symlink must fail.
	if _, _, _, err := store.OpenFile(root, "escape"); err == nil {
		t.Fatal("OpenFile via escaping symlink succeeded, want error")
	}

	// And it must not appear as a usable entry in the listing.
	nodes, _, err := store.ListDir(root, "", 100)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	for _, n := range nodes {
		if n.Name == "escape" {
			t.Fatal("escaping symlink was listed, want it dropped")
		}
	}
}

func TestSymlinkInsideWorkspaceIsAllowed(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()

	link := filepath.Join(root, "alias.go")
	if err := os.Symlink(filepath.Join(root, "src", "app.go"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	rc, _, _, err := store.OpenFile(root, "alias.go")
	if err != nil {
		t.Fatalf("OpenFile via in-workspace symlink: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "package main" {
		t.Fatalf("unexpected content %q", data)
	}
}

func TestSearchFindsNestedFile(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()

	results, _, err := store.Search(root, "app", 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var found bool
	for _, n := range results {
		if n.Path == "src/app.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to find src/app.go, got %v", results)
	}
}

func TestSearchDropsSymlinkThatEscapesWorkspace(t *testing.T) {
	root, secret := setupWorkspace(t)
	store := NewWorkspaceFileStore()
	if err := os.Symlink(secret, filepath.Join(root, "outside-match.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	results, _, err := store.Search(root, "outside", 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, result := range results {
		if result.Name == "outside-match.txt" {
			t.Fatal("escaping symlink appeared in search results")
		}
	}
}

func TestWriteArchiveIncludesWorkspaceFilesAndDropsEscapingSymlinks(t *testing.T) {
	root, secret := setupWorkspace(t)
	store := NewWorkspaceFileStore()
	if err := os.Symlink(secret, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var destination bytes.Buffer
	if err := store.WriteArchive(root, "", &destination); err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(destination.Bytes()), int64(destination.Len()))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	names := make(map[string]bool, len(archive.File))
	for _, file := range archive.File {
		names[file.Name] = true
	}
	if !names[".env"] || !names["src/app.go"] {
		t.Fatalf("archive is missing workspace files: %v", names)
	}
	if names["escape.txt"] {
		t.Fatal("archive included symlink that escapes the workspace")
	}
}

func TestWriteArchiveIncludesAbsoluteSymlinkWithinWorkspace(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()
	if err := os.Symlink(filepath.Join(root, "src", "app.go"), filepath.Join(root, "alias.go")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var destination bytes.Buffer
	if err := store.WriteArchive(root, "", &destination); err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(destination.Bytes()), int64(destination.Len()))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	for _, file := range archive.File {
		if file.Name != "alias.go" {
			continue
		}
		content, err := file.Open()
		if err != nil {
			t.Fatalf("open alias.go: %v", err)
		}
		data, readErr := io.ReadAll(content)
		closeErr := content.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatalf("read alias.go: %v", err)
		}
		if string(data) != "package main" {
			t.Fatalf("alias.go content = %q", data)
		}
		return
	}
	t.Fatal("archive omitted in-workspace absolute symlink")
}

func TestWriteArchiveReturnsDestinationFailure(t *testing.T) {
	root, _ := setupWorkspace(t)
	store := NewWorkspaceFileStore()
	wantErr := errors.New("destination failed")

	err := store.WriteArchive(root, "", failingWriter{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteArchive error = %v, want %v", err, wantErr)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}
