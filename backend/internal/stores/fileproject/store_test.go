package fileproject

import (
	"context"
	"testing"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func TestStoreCreatesUniqueSlugsAndLooksUpBySlug(t *testing.T) {
	store, err := NewWithWorkspaceRoot(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.Create(context.Background(), serviceproject.Meta{
		Name:   "My Project",
		Slug:   serviceproject.Slugify("My Project"),
		Status: serviceproject.StatusProvisioning,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(context.Background(), serviceproject.Meta{
		Name:   "My Project",
		Slug:   serviceproject.Slugify("My Project"),
		Status: serviceproject.StatusProvisioning,
	})
	if err != nil {
		t.Fatal(err)
	}

	if first.Slug != "my-project" {
		t.Fatalf("first slug = %q", first.Slug)
	}
	if second.Slug != "my-project-2" {
		t.Fatalf("second slug = %q", second.Slug)
	}
	if first.Cwd == "" || second.Cwd == "" || first.Cwd == second.Cwd {
		t.Fatalf("unexpected cwd values: first=%q second=%q", first.Cwd, second.Cwd)
	}

	got, err := store.GetBySlug(context.Background(), first.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first.ID {
		t.Fatalf("lookup id = %q, want %q", got.ID, first.ID)
	}

	if err := store.Delete(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetBySlug(context.Background(), first.Slug); err == nil {
		t.Fatal("expected deleted slug lookup to fail")
	}
}
