package chat

import (
	"context"
	"testing"
)

type forkRepository struct {
	Repository
	events  []Event
	copied  []Event
	source  Meta
	created Meta
}

type forkSessionPolicy map[string]bool

func (p forkSessionPolicy) SupportsNativeFork(provider string) bool {
	return p[provider]
}

func (r *forkRepository) Get(context.Context, ID) (Meta, error) {
	if r.source.ID != "" {
		return r.source, nil
	}
	return Meta{ID: "deadbeef", Title: "Source", Provider: "codex"}, nil
}

func (r *forkRepository) ReadEvents(context.Context, ID) ([]Event, error) {
	return append([]Event(nil), r.events...), nil
}

func (r *forkRepository) Create(_ context.Context, meta Meta) (Meta, error) {
	meta.ID = "fadecafe"
	r.created = meta
	return meta, nil
}

func TestForkClearsSessionForProviderWithoutNativeFork(t *testing.T) {
	repo := &forkRepository{source: Meta{
		ID:       "deadbeef",
		Title:    "Source",
		Provider: ProviderAntigravity,
		Sessions: SessionIDs{
			ProviderAntigravity: "agy-session",
			"future-agent":      "future-session",
		},
	}}
	service := New(repo, nil, nil, nil, WithSessionPolicy(forkSessionPolicy{}))

	forked, err := service.Fork(context.Background(), "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if forked.ForkPending || forked.SessionID(ProviderAntigravity) != "" ||
		forked.SessionID("future-agent") != "future-session" {
		t.Fatalf("forked session state = %#v", forked)
	}
	if forked.AntigravitySessionID != "" {
		t.Fatalf("legacy Antigravity session = %q", forked.AntigravitySessionID)
	}
}

func TestForkRetainsSessionOnlyForNativeForkProvider(t *testing.T) {
	repo := &forkRepository{source: Meta{
		ID:       "deadbeef",
		Title:    "Source",
		Provider: ProviderCodex,
		Sessions: SessionIDs{ProviderCodex: "codex-session"},
	}}
	service := New(repo, nil, nil, nil, WithSessionPolicy(forkSessionPolicy{string(ProviderCodex): true}))

	forked, err := service.Fork(context.Background(), "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if !forked.ForkPending || forked.SessionID(ProviderCodex) != "codex-session" ||
		forked.CodexSessionID != "codex-session" {
		t.Fatalf("native fork session state = %#v", forked)
	}
}

func (r *forkRepository) AppendEvent(_ context.Context, _ ID, event Event) (Event, error) {
	return event, nil
}

func (r *forkRepository) AppendCopiedEvent(_ context.Context, _ ID, event Event) (Event, error) {
	r.copied = append(r.copied, event)
	return event, nil
}

func TestForkAppendsEveryHistoryEventThroughTheCopiedEventPort(t *testing.T) {
	repo := &forkRepository{events: []Event{
		{Seq: 1, Type: "tool_use_start", Name: "AskUserQuestion"},
		{Seq: 2, Type: "complete"},
		{Seq: 3, Type: "error", Message: "old failure"},
	}}
	service := New(repo, nil, nil, nil, WithCopiedEventAppender(repo))

	if _, err := service.Fork(context.Background(), "deadbeef"); err != nil {
		t.Fatal(err)
	}
	if len(repo.copied) != len(repo.events) {
		t.Fatalf("copied %d events, want %d", len(repo.copied), len(repo.events))
	}
	for index, event := range repo.copied {
		if event.Seq != 0 {
			t.Fatalf("copied event %d retained sequence %d", index, event.Seq)
		}
	}
}
