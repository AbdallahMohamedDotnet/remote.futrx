package fileapplicationinstalls

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
)

func TestRunPersistenceSeparatesScopesAndProjects(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store := New(dir)
	run := svc.InstallRun{ID: "run", ApplicationID: "../app", Scope: svc.ScopeProject, ProjectID: "../project", State: "failed", Log: "connection reset"}
	if err := store.Put(ctx, run); err != nil {
		t.Fatal(err)
	}
	reopened := New(dir)
	got, found, err := reopened.Get(ctx, run.Scope, run.ProjectID, run.ApplicationID)
	if err != nil || !found || got != run {
		t.Fatalf("reopened=%+v %v %v", got, found, err)
	}
	for _, scope := range []svc.Scope{svc.ScopeGlobal, svc.ScopeProject} {
		if _, found, err := reopened.Get(ctx, scope, "different-project", run.ApplicationID); err != nil || found {
			t.Fatalf("another scope/project exposed the log: %v %v", found, err)
		}
	}
	files, err := os.ReadDir(filepath.Join(dir, "application-installs"))
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v %v", files, err)
	}
	info, _ := files[0].Info()
	if info.Mode().Perm() != 0600 {
		t.Fatalf("run mode=%v", info.Mode())
	}
}
