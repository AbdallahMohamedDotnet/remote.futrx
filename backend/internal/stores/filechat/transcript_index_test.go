package filechat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

	var indexedEvents, indexedTurns int
	if err := store.index.db.QueryRow(
		"SELECT count(*) FROM chat_event_offsets WHERE chat_id = ?", "abcd",
	).Scan(&indexedEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.index.db.QueryRow(
		"SELECT count(*) FROM chat_transcript_turns WHERE chat_id = ?", "abcd",
	).Scan(&indexedTurns); err != nil {
		t.Fatal(err)
	}
	if indexedEvents != 9 || indexedTurns != 3 {
		t.Fatalf("indexed events = %d, turns = %d", indexedEvents, indexedTurns)
	}

	locations, err := store.index.transcriptLocations(context.Background(), "abcd", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 6 {
		t.Fatalf("bounded window has %d events, want the newest two turns (6)", len(locations))
	}
	if locations[0].seq != 4 || locations[len(locations)-1].seq != 9 {
		t.Fatalf("bounded window locations = %#v", locations)
	}
	if _, err := os.Stat(filepath.Join(root, transcriptIndexFilename)); err != nil {
		t.Fatalf("transcript index was not created: %v", err)
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

	state, found, err := store.index.readState(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if !found || state.eventOrdinal != 4 || state.lastSeq != 4 {
		t.Fatalf("incremental index state = %#v, found = %t", state, found)
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
	var turns int
	if err := store.index.db.QueryRow(
		"SELECT count(*) FROM chat_transcript_turns WHERE chat_id = ?", "abcd",
	).Scan(&turns); err != nil {
		t.Fatal(err)
	}
	if turns != 1 {
		t.Fatalf("rewound turn rows = %d, want 1", turns)
	}

	if err := store.Delete(context.Background(), "abcd"); err != nil {
		t.Fatal(err)
	}
	var states int
	if err := store.index.db.QueryRow(
		"SELECT count(*) FROM chat_event_index_state WHERE chat_id = ?", "abcd",
	).Scan(&states); err != nil {
		t.Fatal(err)
	}
	if states != 0 {
		t.Fatalf("deleted chat retained %d index state rows", states)
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
	if err := store.index.db.Close(); err != nil {
		t.Fatal(err)
	}

	appended, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
		T: 3, Type: "user", TurnID: "turn-2", Text: "second",
	})
	if err != nil {
		t.Fatal(err)
	}
	if appended.Seq != 3 {
		t.Fatalf("fallback append sequence = %d, want 3", appended.Seq)
	}

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
	t.Cleanup(func() { _ = store.Close() })
	return store
}
