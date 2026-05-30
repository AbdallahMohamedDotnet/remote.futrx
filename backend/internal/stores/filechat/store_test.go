package filechat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

func TestStoreListUsesCachedMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "chats", "abcd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "meta.json"),
		[]byte(`{"id":"abcd","title":"Existing","createdAt":1,"lastMessageAt":10}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	list, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "abcd" || list[0].Title != "Existing" {
		t.Fatalf("loaded list = %#v", list)
	}

	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "beef", Title: "New", CreatedAt: 2, LastMessageAt: 20}); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "beef" {
		t.Fatalf("created list = %#v", list)
	}

	if _, err := store.Update(context.Background(), "abcd", func(m *servicechat.Meta) {
		m.Title = "Renamed"
		m.LastMessageAt = 30
	}); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "abcd" || list[0].Title != "Renamed" {
		t.Fatalf("updated list = %#v", list)
	}

	if _, err := store.AppendEvent(context.Background(), "beef", servicechat.Event{T: 40, Type: "user", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "beef" || list[0].LastMessageAt != 40 {
		t.Fatalf("append list = %#v", list)
	}

	if err := store.Delete(context.Background(), "beef"); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "abcd" {
		t.Fatalf("deleted list = %#v", list)
	}
}

func TestStoreReadsEventPages(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
			T:    int64(i),
			Type: "user",
			Text: string(rune('a' + i - 1)),
		}); err != nil {
			t.Fatal(err)
		}
	}

	page, err := store.ReadEventsPage(context.Background(), "abcd", servicechat.EventPageQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.Events[0].Seq != 4 || page.Events[1].Seq != 5 {
		t.Fatalf("latest page = %#v", page)
	}
	if !page.HasMore || page.NextBefore != 4 || page.LastSeq != 5 {
		t.Fatalf("latest page cursors = %#v", page)
	}

	older, err := store.ReadEventsPage(context.Background(), "abcd", servicechat.EventPageQuery{Limit: 2, BeforeSeq: page.NextBefore})
	if err != nil {
		t.Fatal(err)
	}
	if len(older.Events) != 2 || older.Events[0].Seq != 2 || older.Events[1].Seq != 3 {
		t.Fatalf("older page = %#v", older)
	}

	after, err := store.ReadEventsAfter(context.Background(), "abcd", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].Seq != 4 || after[1].Seq != 5 {
		t.Fatalf("after = %#v", after)
	}
}
