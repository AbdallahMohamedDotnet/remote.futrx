package filechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

func TestTranscriptIndexBackfillsExistingChatAndReadsBoundedTurnWindow(t *testing.T) {
	root := t.TempDir()
	events := []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", TurnID: "turn-1", Text: "first"},
		{Seq: 2, T: 2, Type: "assistant_text", TurnID: "turn-1", Text: "one"},
		{Seq: 3, T: 3, Type: "complete", TurnID: "turn-1"},
		{Seq: 4, T: 4, Type: "user", TurnID: "turn-2", Text: "second"},
		{Seq: 5, T: 5, Type: "assistant_text", TurnID: "turn-2", Text: "two"},
		{Seq: 6, T: 6, Type: "complete", TurnID: "turn-2"},
		{Seq: 7, T: 7, Type: "user", TurnID: "turn-3", Text: "third"},
		{Seq: 8, T: 8, Type: "assistant_text", TurnID: "turn-3", Text: "three"},
		{Seq: 9, T: 9, Type: "complete", TurnID: "turn-3"},
	}
	writeStoredChat(t, root, "abcd", events)

	store := newIndexedTestStore(t, root)
	service := servicechat.New(
		store,
		nil,
		nil,
		nil,
		servicechat.WithTranscriptEventSource(store),
		servicechat.WithTranscriptEventWindowSource(store),
	)
	page, err := service.TranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 || page.Turns[0].ID != "turn-3" ||
		page.NextBefore != 7 || !page.HasMore || page.LastSeq != 9 {
		t.Fatalf("latest indexed page = %#v", page)
	}

	older, err := service.TranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1, BeforeSeq: page.NextBefore},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(older.Turns) != 1 || older.Turns[0].ID != "turn-2" ||
		older.NextBefore != 4 || !older.HasMore || older.LastSeq != 9 {
		t.Fatalf("older indexed page = %#v", older)
	}

	state, found := store.index.snapshot("abcd")
	if !found {
		t.Fatal("chat index was not cached")
	}
	if len(state.events) != 9 || len(state.turns) != 3 {
		t.Fatalf("indexed events = %d, turns = %d", len(state.events), len(state.turns))
	}

	locations, err := transcriptLocations(context.Background(), state, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 6 {
		t.Fatalf("bounded window has %d events, want the newest two turns (6)", len(locations))
	}
	if locations[0].seq != 4 || locations[len(locations)-1].seq != 9 {
		t.Fatalf("bounded window locations = %#v", locations)
	}
	if _, err := os.Stat(filepath.Join(root, "transcript-index.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("durable transcript index should not exist: %v", err)
	}
}

func TestTranscriptIndexIncrementallyRepairsOutOfBandAppend(t *testing.T) {
	root := t.TempDir()
	store := newIndexedTestStore(t, root)
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []servicechat.Event{
		{T: 1, Type: "user", TurnID: "turn-1", Text: "first"},
		{T: 2, Type: "complete", TurnID: "turn-1"},
	} {
		if _, err := store.AppendEvent(context.Background(), "abcd", event); err != nil {
			t.Fatal(err)
		}
	}

	appendStoredEvent(t, store.eventsPath("abcd"), servicechat.Event{
		Seq: 3, T: 3, Type: "user", TurnID: "turn-2", Text: "external",
	})
	appended, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
		T: 4, Type: "complete", TurnID: "turn-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if appended.Seq != 4 {
		t.Fatalf("appended sequence = %d, want 4", appended.Seq)
	}

	after, err := store.ReadEventsAfter(context.Background(), "abcd", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].Seq != 3 || after[1].Seq != 4 {
		t.Fatalf("events after indexed cursor = %#v", after)
	}

	state, found := store.index.snapshot("abcd")
	if !found || state.eventOrdinal != 4 || state.lastSeq != 4 {
		t.Fatalf("incremental index state = %#v, found = %t", state, found)
	}
}

func TestTranscriptIndexRebuildsLazilyAfterStoreRestart(t *testing.T) {
	root := t.TempDir()
	store := newIndexedTestStore(t, root)
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []servicechat.Event{
		{T: 1, Type: "user", TurnID: "turn-1", Text: "first"},
		{T: 2, Type: "complete", TurnID: "turn-1"},
	} {
		if _, err := store.AppendEvent(context.Background(), "abcd", event); err != nil {
			t.Fatal(err)
		}
	}
	if _, found := store.index.snapshot("abcd"); !found {
		t.Fatal("first store did not cache the chat index")
	}

	reopened := newIndexedTestStore(t, root)
	if _, found := reopened.index.snapshot("abcd"); found {
		t.Fatal("new store unexpectedly inherited process-local index state")
	}
	page, err := reopened.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.LastSeq != 2 {
		t.Fatalf("reopened page = %#v", page)
	}
	state, found := reopened.index.snapshot("abcd")
	if !found || len(state.events) != 2 || len(state.turns) != 1 {
		t.Fatalf("rebuilt index state = %#v, found = %t", state, found)
	}
}

func TestTranscriptIndexRebuildsAfterSameSizeRewrite(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "abcd", []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", TurnID: "turn-1", Text: "first"},
	})
	store := newIndexedTestStore(t, root)
	page, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Text != "first" {
		t.Fatalf("initial page = %#v", page)
	}

	path := store.eventsPath("abcd")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := bytes.Replace(data, []byte(`"first"`), []byte(`"other"`), 1)
	if len(rewritten) != len(data) || bytes.Equal(rewritten, data) {
		t.Fatal("test rewrite must change content without changing file size")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, rewritten, 0o644); err != nil {
		t.Fatal(err)
	}
	changed := info.ModTime().Add(time.Second)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}

	page, err = store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Text != "other" {
		t.Fatalf("rewritten page = %#v", page)
	}
}

func TestTranscriptIndexRebuildsWhenLargerRewriteChangesIndexedPrefix(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "abcd", []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", TurnID: "turn-1", Text: "old"},
		{Seq: 2, T: 2, Type: "complete", TurnID: "turn-1"},
	})
	store := newIndexedTestStore(t, root)
	if _, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	); err != nil {
		t.Fatal(err)
	}

	writeStoredChat(t, root, "abcd", []servicechat.Event{
		{Seq: 10, T: 10, Type: "user", TurnID: "replacement", Text: "new and longer"},
		{Seq: 11, T: 11, Type: "assistant_text", TurnID: "replacement", Text: "answer"},
		{Seq: 12, T: 12, Type: "complete", TurnID: "replacement"},
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	appended, err := store.AppendEvent(canceled, "abcd", servicechat.Event{
		T: 13, Type: "assistant_text", TurnID: "replacement-2", Text: "after rewrite",
	})
	if err != nil {
		t.Fatal(err)
	}
	if appended.Seq != 13 {
		t.Fatalf("appended sequence after rewrite = %d, want 13", appended.Seq)
	}
	page, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 4 || page.Events[0].Seq != 10 ||
		page.Events[0].Text != "new and longer" || page.Events[3].Text != "after rewrite" ||
		page.LastSeq != 13 {
		t.Fatalf("page after larger rewrite = %#v", page)
	}
}

func TestChatLockRemainsStableAcrossDeleteAndRecreate(t *testing.T) {
	store := newIndexedTestStore(t, t.TempDir())
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	before := store.lock("abcd")
	if err := store.Delete(context.Background(), "abcd"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	after := store.lock("abcd")
	if before != after {
		t.Fatal("delete and recreate replaced the per-chat mutex")
	}
}

func TestTranscriptIndexEvictsLeastRecentlyUsedChats(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "aaaa", []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", TurnID: "turn-a"},
	})
	writeStoredChat(t, root, "bbbb", []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", TurnID: "turn-b"},
	})
	store := newIndexedTestStore(t, root)
	store.index.maxLocations = 2

	if _, err := store.ReadEventsPage(
		context.Background(),
		"aaaa",
		servicechat.EventPageQuery{Limit: 10},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEventsPage(
		context.Background(),
		"bbbb",
		servicechat.EventPageQuery{Limit: 10},
	); err != nil {
		t.Fatal(err)
	}
	if _, found := store.index.snapshot("aaaa"); found {
		t.Fatal("least recently used chat was not evicted")
	}
	if _, found := store.index.snapshot("bbbb"); !found {
		t.Fatal("current chat was evicted")
	}

	if _, err := store.ReadEventsPage(
		context.Background(),
		"aaaa",
		servicechat.EventPageQuery{Limit: 10},
	); err != nil {
		t.Fatal(err)
	}
	if _, found := store.index.snapshot("aaaa"); !found {
		t.Fatal("evicted chat was not rebuilt on demand")
	}
}

func TestTranscriptIndexBoundsCachedEmptyChats(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "aaaa", nil)
	writeStoredChat(t, root, "bbbb", nil)
	store := newIndexedTestStore(t, root)
	store.index.maxChats = 1

	for _, id := range []servicechat.ID{"aaaa", "bbbb"} {
		if _, err := store.ReadEventsPage(
			context.Background(),
			id,
			servicechat.EventPageQuery{Limit: 10},
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, found := store.index.snapshot("aaaa"); found {
		t.Fatal("least recently used empty chat was not evicted")
	}
	if _, found := store.index.snapshot("bbbb"); !found {
		t.Fatal("current empty chat was evicted")
	}
}

func TestNewRemovesLegacyTranscriptIndexFiles(t *testing.T) {
	root := t.TempDir()
	legacy := []string{
		"transcript-index.sqlite",
		"transcript-index.sqlite-wal",
		"transcript-index.sqlite-shm",
	}
	for _, name := range legacy {
		if err := os.WriteFile(filepath.Join(root, name), []byte("disposable"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	newIndexedTestStore(t, root)
	for _, name := range legacy {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy index %q still exists: %v", name, err)
		}
	}
}

func TestTranscriptIndexRebuildsWhenAnIncompleteTailGrows(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "abcd", nil)
	path := filepath.Join(root, "chats", "abcd", "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"t":1,"type":"user","text":"hel`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newIndexedTestStore(t, root)
	page, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 0 || page.LastSeq != 0 {
		t.Fatalf("incomplete page = %#v", page)
	}
	state, found := store.index.snapshot("abcd")
	if !found || state.tailComplete {
		t.Fatalf("incomplete tail state = %#v, found = %t", state, found)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("lo\"}\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	page, err = store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Seq != 1 || page.Events[0].Text != "hello" {
		t.Fatalf("completed page = %#v", page)
	}
	state, found = store.index.snapshot("abcd")
	if !found || !state.tailComplete || state.eventOrdinal != 1 {
		t.Fatalf("rebuilt tail state = %#v, found = %t", state, found)
	}
}

func TestTranscriptIndexDoesNotPublishCanceledBuild(t *testing.T) {
	root := t.TempDir()
	writeStoredChat(t, root, "abcd", []servicechat.Event{
		{Seq: 1, T: 1, Type: "user", Text: "first"},
	})
	store := newIndexedTestStore(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.index.syncChat(ctx, "abcd", store.eventsPath("abcd")); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync error = %v, want context.Canceled", err)
	}
	if _, found := store.index.snapshot("abcd"); found {
		t.Fatal("canceled build published partial state")
	}
}

func TestTranscriptIndexSupportsConcurrentChatReadsAndAppends(t *testing.T) {
	store := newIndexedTestStore(t, t.TempDir())
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}

	const eventCount = 100
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 1; i <= eventCount; i++ {
			_, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
				T: int64(i), Type: "user", Text: fmt.Sprintf("event-%d", i),
			})
			if err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for range eventCount {
			if _, err := store.ReadEventsPage(
				context.Background(),
				"abcd",
				servicechat.EventPageQuery{Limit: 20},
			); err != nil {
				errs <- err
				return
			}
		}
	}()
	workers.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	page, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: eventCount},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != eventCount || page.LastSeq != eventCount {
		t.Fatalf("final concurrent page = %#v", page)
	}
}

func TestTranscriptIndexMatchesLegacySequenceAndTurnRules(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "chats", "abcd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "meta.json"),
		[]byte(`{"id":"abcd","title":"Legacy","createdAt":1,"lastMessageAt":5}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	content := "" +
		`{"t":1,"type":"user","text":"question"}` + "\n" +
		"\n" +
		"{invalid-json}\n" +
		`{"t":3,"type":"assistant_text","text":"answer"}` + "\n" +
		`{"t":4,"type":"user","text":"next"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newIndexedTestStore(t, root)
	window, err := store.ReadTranscriptEventWindow(context.Background(), "abcd", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if window.LastSeq != 4 || len(window.Events) != 3 {
		t.Fatalf("legacy indexed window = %#v", window)
	}
	if window.Events[0].Seq != 1 || window.Events[1].Seq != 3 || window.Events[2].Seq != 4 {
		t.Fatalf("legacy fallback sequences = %#v", window.Events)
	}

	service := servicechat.New(
		store,
		nil,
		nil,
		nil,
		servicechat.WithTranscriptEventSource(store),
		servicechat.WithTranscriptEventWindowSource(store),
	)
	page, err := service.TranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 || page.Turns[0].StartSeq != 4 ||
		page.NextBefore != 4 || !page.HasMore {
		t.Fatalf("legacy indexed transcript = %#v", page)
	}
}

func TestTranscriptIndexRebuildsAfterRewindAndCleansUpAfterDelete(t *testing.T) {
	root := t.TempDir()
	store := newIndexedTestStore(t, root)
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []servicechat.Event{
		{T: 10, Type: "user", TurnID: "turn-1", Text: "keep"},
		{T: 20, Type: "complete", TurnID: "turn-1"},
		{T: 30, Type: "user", TurnID: "turn-2", Text: "remove"},
		{T: 40, Type: "complete", TurnID: "turn-2"},
	} {
		if _, err := store.AppendEvent(context.Background(), "abcd", event); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.TruncateEventsBefore(context.Background(), "abcd", 30); err != nil {
		t.Fatal(err)
	}

	window, err := store.ReadTranscriptEventWindow(context.Background(), "abcd", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if window.LastSeq != 2 || len(window.Events) != 2 || window.Events[0].Text != "keep" {
		t.Fatalf("rewound indexed window = %#v", window)
	}
	state, found := store.index.snapshot("abcd")
	if !found || len(state.turns) != 1 {
		t.Fatalf("rewound index state = %#v, found = %t", state, found)
	}

	if err := store.Delete(context.Background(), "abcd"); err != nil {
		t.Fatal(err)
	}
	if _, found := store.index.snapshot("abcd"); found {
		t.Fatal("deleted chat retained cached index state")
	}
}

func TestTranscriptIndexFailuresFallBackToCanonicalEventLog(t *testing.T) {
	root := t.TempDir()
	store := newIndexedTestStore(t, root)
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []servicechat.Event{
		{T: 1, Type: "user", TurnID: "turn-1", Text: "first"},
		{T: 2, Type: "complete", TurnID: "turn-1"},
	} {
		if _, err := store.AppendEvent(context.Background(), "abcd", event); err != nil {
			t.Fatal(err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	appended, err := store.AppendEvent(canceled, "abcd", servicechat.Event{
		T: 3, Type: "user", TurnID: "turn-2", Text: "second",
	})
	if err != nil {
		t.Fatal(err)
	}
	if appended.Seq != 3 {
		t.Fatalf("fallback append sequence = %d, want 3", appended.Seq)
	}
	state, found := store.index.snapshot("abcd")
	if !found || len(state.events) != 3 {
		t.Fatalf("refreshed index state = %#v, found = %t", state, found)
	}
	state.events = append([]indexedEventLocation(nil), state.events...)
	state.turns = append([]indexedTurnRange(nil), state.turns...)
	state.events[len(state.events)-1].length = maxEventRecordBytes + 3
	store.index.publish("abcd", state)

	page, err := store.ReadEventsPage(
		context.Background(),
		"abcd",
		servicechat.EventPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Seq != 3 ||
		page.NextBefore != 3 || page.LastSeq != 3 || !page.HasMore {
		t.Fatalf("fallback event page = %#v", page)
	}

	after, err := store.ReadEventsAfter(context.Background(), "abcd", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].Seq != 2 || after[1].Seq != 3 {
		t.Fatalf("fallback events after cursor = %#v", after)
	}

	window, err := store.ReadTranscriptEventWindow(context.Background(), "abcd", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Events) != 3 || window.LastSeq != 3 {
		t.Fatalf("fallback transcript window = %#v", window)
	}
}

func BenchmarkTranscriptIndexLatestPage(b *testing.B) {
	root := b.TempDir()
	events := make([]servicechat.Event, 0, 15_000)
	for turn := 1; turn <= 5_000; turn++ {
		turnID := fmt.Sprintf("turn-%d", turn)
		events = append(events,
			servicechat.Event{Type: "user", TurnID: turnID, Text: "question"},
			servicechat.Event{Type: "assistant_text", TurnID: turnID, Text: "answer"},
			servicechat.Event{Type: "complete", TurnID: turnID},
		)
	}
	for i := range events {
		events[i].Seq = int64(i + 1)
		events[i].T = int64(i + 1)
	}
	writeStoredChat(b, root, "abcd", events)

	store := newIndexedTestStore(b, root)
	service := servicechat.New(
		store,
		nil,
		nil,
		nil,
		servicechat.WithTranscriptEventSource(store),
		servicechat.WithTranscriptEventWindowSource(store),
	)
	if _, err := service.TranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 20},
	); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		if _, err := service.TranscriptPage(
			context.Background(),
			"abcd",
			servicechat.TranscriptPageQuery{Limit: 20},
		); err != nil {
			b.Fatal(err)
		}
	}
}

func writeStoredChat(
	t testing.TB,
	root string,
	id servicechat.ID,
	events []servicechat.Event,
) {
	t.Helper()
	dir := filepath.Join(root, "chats", string(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(metaRecordFromDomain(servicechat.Meta{
		ID: id, Title: "Stored", CreatedAt: 1, LastMessageAt: int64(len(events)),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(
		filepath.Join(dir, "events.jsonl"),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(eventRecordFromDomain(event)); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func appendStoredEvent(t testing.TB, path string, event servicechat.Event) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(eventRecordFromDomain(event)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func newIndexedTestStore(t testing.TB, root string) *Store {
	t.Helper()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
