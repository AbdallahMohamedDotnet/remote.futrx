package filechat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
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
	if list[0].LastReadAt != 10 {
		t.Fatalf("legacy chat should start read, got lastReadAt=%d", list[0].LastReadAt)
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

func TestStoreReadsTranscriptPagesByWholeTurnAndCompactsDeltas(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}

	appendChatEvents(t, store, "abcd",
		servicechat.Event{T: 1, Type: "user", Text: "older question"},
		servicechat.Event{T: 2, Type: "assistant_text", Text: "older answer"},
		servicechat.Event{T: 3, Type: "complete"},
		servicechat.Event{T: 4, Type: "user", TurnID: "turn-new", Text: "new question"},
	)
	for i := 0; i < 300; i++ {
		appendChatEvents(t, store, "abcd", servicechat.Event{
			T:         int64(5 + i),
			Type:      "assistant_text",
			TurnID:    "turn-new",
			MessageID: "message-new",
			Text:      "x",
		})
	}
	appendChatEvents(t, store, "abcd", servicechat.Event{
		T: 305, Type: "complete", TurnID: "turn-new",
	})

	page, err := store.ReadTranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 {
		t.Fatalf("latest transcript page = %#v", page)
	}
	turn := page.Turns[0]
	if turn.ID != "turn-new" || turn.StartSeq != 4 || turn.EndSeq != 305 {
		t.Fatalf("latest turn boundaries = %#v", turn)
	}
	if len(turn.Events) != 3 || turn.Events[0].Type != "user" ||
		turn.Events[1].Type != "assistant_text" || turn.Events[2].Type != "complete" {
		t.Fatalf("compacted turn events = %#v", turn.Events)
	}
	if turn.Events[1].Text != strings.Repeat("x", 300) {
		t.Fatalf("assistant text length = %d, want 300", len(turn.Events[1].Text))
	}
	if !page.HasMore || page.NextBefore != 4 || page.LastSeq != 305 {
		t.Fatalf("latest transcript cursors = %#v", page)
	}

	older, err := store.ReadTranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1, BeforeSeq: page.NextBefore},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(older.Turns) != 1 || older.Turns[0].StartSeq != 1 || older.Turns[0].EndSeq != 3 {
		t.Fatalf("older transcript page = %#v", older)
	}
	if older.HasMore || older.NextBefore != 0 || older.LastSeq != 305 {
		t.Fatalf("older transcript cursors = %#v", older)
	}
}

func TestStoreTranscriptKeepsIncompleteTurnAndToolLifecycleTogether(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}

	appendChatEvents(t, store, "abcd",
		servicechat.Event{T: 1, Type: "user", TurnID: "turn-old", Text: "old"},
		servicechat.Event{T: 2, Type: "assistant_text", TurnID: "turn-old", Text: "done"},
		servicechat.Event{T: 3, Type: "complete", TurnID: "turn-old"},
		servicechat.Event{T: 4, Type: "user", TurnID: "turn-live", Text: "inspect"},
		servicechat.Event{T: 5, Type: "assistant_text", TurnID: "turn-live", Text: "before"},
		servicechat.Event{T: 6, Type: "tool_use_start", TurnID: "turn-live", ID: "tool-1", Name: "Bash"},
	)
	for i := 0; i < 260; i++ {
		appendChatEvents(t, store, "abcd", servicechat.Event{
			T: int64(7 + i), Type: "thinking", TurnID: "turn-live", MessageID: "reasoning-1", Text: "r",
		})
	}
	appendChatEvents(t, store, "abcd",
		servicechat.Event{T: 267, Type: "tool_use_end", TurnID: "turn-live", ID: "tool-1", Output: "ok"},
		servicechat.Event{T: 268, Type: "assistant_text", TurnID: "turn-live", Text: "after"},
	)

	page, err := store.ReadTranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 || page.Turns[0].ID != "turn-live" {
		t.Fatalf("incomplete page = %#v", page)
	}
	events := page.Turns[0].Events
	if len(events) != 6 {
		t.Fatalf("incomplete compacted events = %#v", events)
	}
	if events[2].Type != "tool_use_start" || events[2].ID != "tool-1" ||
		events[3].Type != "thinking" || events[3].Text != strings.Repeat("r", 260) ||
		events[4].Type != "tool_use_end" || events[4].ID != "tool-1" || events[4].Output != "ok" {
		t.Fatalf("tool lifecycle was not preserved: %#v", events)
	}
	if events[len(events)-1].Type != "assistant_text" || events[len(events)-1].Text != "after" {
		t.Fatalf("incomplete assistant tail = %#v", events[len(events)-1])
	}
	if !page.HasMore || page.NextBefore != 4 {
		t.Fatalf("incomplete page cursor = %#v", page)
	}
}

func TestStoreTranscriptTreatsLegacyOrphanEventsAsOneSafePage(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		appendChatEvents(t, store, "abcd", servicechat.Event{
			T: int64(i + 1), Type: "assistant_text", Text: "x",
		})
	}

	page, err := store.ReadTranscriptPage(
		context.Background(),
		"abcd",
		servicechat.TranscriptPageQuery{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 || len(page.Turns[0].Events) != 1 ||
		page.Turns[0].Events[0].Text != strings.Repeat("x", 300) {
		t.Fatalf("legacy orphan transcript = %#v", page)
	}
	if page.HasMore || page.NextBefore != 0 {
		t.Fatalf("orphan transcript must not expose an unsafe cursor: %#v", page)
	}
}

func appendChatEvents(
	t *testing.T,
	store *Store,
	chatID servicechat.ID,
	events ...servicechat.Event,
) {
	t.Helper()
	for _, event := range events {
		if _, err := store.AppendEvent(context.Background(), chatID, event); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStoreRewindClearsProviderSessionIDs(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{
		ID:            "abcd",
		CreatedAt:     10,
		LastMessageAt: 10,
		Sessions: servicechat.SessionIDs{
			servicechat.ProviderAntigravity: "agy-session",
			"future-agent":                  "future-session",
		},
		ClaudeSessionID: "claude-session",
		CodexSessionID:  "codex-session",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
		T:    20,
		Type: "user",
		Text: "keep",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
		T:    30,
		Type: "user",
		Text: "rewind from here",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{
		T:    40,
		Type: "assistant_text",
		Text: "remove",
	}); err != nil {
		t.Fatal(err)
	}

	kept, err := store.TruncateEventsBefore(context.Background(), "abcd", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].Text != "keep" {
		t.Fatalf("kept events = %#v", kept)
	}

	events, err := store.ReadEvents(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Text != "keep" {
		t.Fatalf("persisted events = %#v", events)
	}

	meta, err := store.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Sessions) != 0 || meta.ClaudeSessionID != "" || meta.CodexSessionID != "" ||
		meta.AntigravitySessionID != "" {
		t.Fatalf("session ids were not cleared: %#v", meta)
	}
	if meta.LastMessageAt != 20 {
		t.Fatalf("LastMessageAt = %d, want 20", meta.LastMessageAt)
	}
}

func TestStorePersistsGenericAndLegacySessionShapes(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(context.Background(), servicechat.Meta{
		ID:       "abcd",
		Provider: servicechat.ProviderAntigravity,
		Sessions: servicechat.SessionIDs{
			servicechat.ProviderAntigravity: "agy-session",
			"future-agent":                  "future-session",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.AntigravitySessionID != "agy-session" {
		t.Fatalf("legacy Antigravity session = %q", created.AntigravitySessionID)
	}
	event := servicechat.Event{T: 10, Type: "session"}
	event.SetSession("future-agent", "event-session")
	if _, err := store.AppendEvent(context.Background(), "abcd", event); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := reopened.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if meta.SessionID(servicechat.ProviderAntigravity) != "agy-session" || meta.SessionID("future-agent") != "future-session" {
		t.Fatalf("reloaded sessions = %#v", meta.Sessions)
	}
	events, err := reopened.ReadEvents(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Provider != "future-agent" || events[0].SessionID != "event-session" {
		t.Fatalf("reloaded events = %#v", events)
	}
}

func TestStoreImportsLegacySessionFields(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "chats", "abcd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "meta.json"),
		[]byte(`{"id":"abcd","title":"Legacy","createdAt":1,"lastMessageAt":1,"antigravitySessionId":"agy-legacy"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "events.jsonl"),
		[]byte("{\"t\":2,\"type\":\"session\",\"kimiSessionId\":\"kimi-legacy\"}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if meta.SessionID(servicechat.ProviderAntigravity) != "agy-legacy" {
		t.Fatalf("legacy metadata sessions = %#v", meta.Sessions)
	}
	events, err := store.ReadEvents(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Provider != servicechat.ProviderKimi || events[0].SessionID != "kimi-legacy" {
		t.Fatalf("legacy event = %#v", events)
	}
}

func TestStorePersistsSelectedSkills(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	created, err := store.Create(context.Background(), servicechat.Meta{
		ID:       "abcd",
		Provider: servicechat.ProviderCodex,
		SelectedSkills: []servicechat.SkillRef{
			{Name: "Custom Skill", Command: "custom", Source: "user"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.SelectedSkills) != 1 || created.SelectedSkills[0].Provider != servicechat.ProviderCodex {
		t.Fatalf("created skills = %#v", created.SelectedSkills)
	}

	loaded, err := store.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SelectedSkills) != 1 || loaded.SelectedSkills[0].Command != "custom" {
		t.Fatalf("loaded skills = %#v", loaded.SelectedSkills)
	}

	updated, err := store.Update(context.Background(), "abcd", func(m *servicechat.Meta) {
		m.SelectedSkills = append(m.SelectedSkills, servicechat.SkillRef{
			Name:     "Review",
			Command:  "review",
			Provider: servicechat.ProviderCodex,
			Source:   "project",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.SelectedSkills) != 2 || updated.SelectedSkills[1].Source != "project" {
		t.Fatalf("updated skills = %#v", updated.SelectedSkills)
	}

	reopened, err := New(store.root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = reopened.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SelectedSkills) != 2 || loaded.SelectedSkills[0].Provider != servicechat.ProviderCodex {
		t.Fatalf("reloaded skills = %#v", loaded.SelectedSkills)
	}
}

func TestStorePersistsAgentSelectionsAcrossInstances(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{
		ID:              "abcd",
		Provider:        servicechat.ProviderClaude,
		Model:           "claude-opus-current",
		Mode:            "plan",
		ReasoningEffort: "high",
		ServiceTier:     "fast",
		ProjectID:       "project-1",
	}); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(store.root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != servicechat.ProviderClaude ||
		loaded.Model != "claude-opus-current" ||
		loaded.Mode != "plan" ||
		loaded.ReasoningEffort != "high" ||
		loaded.ServiceTier != "fast" ||
		loaded.ProjectID != "project-1" {
		t.Fatalf("reloaded selections = %#v", loaded)
	}
}
