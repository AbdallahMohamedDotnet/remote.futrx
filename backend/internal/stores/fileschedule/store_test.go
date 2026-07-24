package fileschedule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	serviceschedule "github.com/futrx-com/remote.futrx.com/internal/service/schedule"
)

const testID serviceschedule.ID = "0123456789abcdef01234567"

func TestStoreCRUDPersistsAcrossReopen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	task := serviceschedule.Task{
		ID:         testID,
		Name:       "watch deploy",
		OwnerEmail: "owner@example.com",
		Kind:       serviceschedule.KindCron,
		Cron:       "* * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}
	created, err := store.Create(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("timestamps were not initialized: %#v", created)
	}

	updated, err := store.Update(context.Background(), testID, func(task *serviceschedule.Task) error {
		task.RunCount = 2
		task.Status = serviceschedule.StatusActive
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RunCount != 2 {
		t.Fatalf("updated task = %#v", updated)
	}

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(context.Background(), testID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != task.Name || got.RunCount != 2 {
		t.Fatalf("reopened task = %#v", got)
	}

	info, err := os.Stat(filepath.Join(root, "scheduled-tasks", "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}

	if err := reopened.Delete(context.Background(), testID); err != nil {
		t.Fatal(err)
	}
	again, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := again.Get(context.Background(), testID); !errors.Is(err, serviceschedule.ErrNotFound) {
		t.Fatalf("deleted task error = %v", err)
	}
}

func TestStoreUpdateFailureDoesNotMutateMemoryOrDisk(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(context.Background(), serviceschedule.Task{ID: testID, Name: "original"})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stop")
	_, err = store.Update(context.Background(), testID, func(task *serviceschedule.Task) error {
		task.Name = "mutated"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("update error = %v", err)
	}
	got, _ := store.Get(context.Background(), testID)
	if got.Name != "original" {
		t.Fatalf("in-memory task mutated: %#v", got)
	}
	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = reopened.Get(context.Background(), testID)
	if got.Name != "original" {
		t.Fatalf("persisted task mutated: %#v", got)
	}
}

func TestStoreSerializesConcurrentUpdates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), serviceschedule.Task{ID: testID}); err != nil {
		t.Fatal(err)
	}

	const updates = 24
	var wg sync.WaitGroup
	errs := make(chan error, updates)
	for range updates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Update(context.Background(), testID, func(task *serviceschedule.Task) error {
				task.RunCount++
				return nil
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, _ := store.Get(context.Background(), testID)
	if got.RunCount != updates {
		t.Fatalf("runCount = %d, want %d", got.RunCount, updates)
	}
	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = reopened.Get(context.Background(), testID)
	if got.RunCount != updates {
		t.Fatalf("persisted runCount = %d, want %d", got.RunCount, updates)
	}
}

func TestStoreHonorsCancelledContextAndValidatesIDs(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v", err)
	}
	if _, err := store.Get(context.Background(), "bad"); !errors.Is(err, serviceschedule.ErrInvalidID) {
		t.Fatalf("Get error = %v", err)
	}
}

func TestNewRejectsCorruptOrUnknownFile(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"corrupt": `{`,
		"version": `{"version":99,"tasks":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "scheduled-tasks")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "tasks.json"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := New(root); err == nil {
				t.Fatal("New succeeded for invalid persistence file")
			}
		})
	}
}
