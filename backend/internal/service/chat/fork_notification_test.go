package chat

import (
	"context"
	"testing"
)

type forkRepository struct {
	Repository
	events       []Event
	suppressions []bool
}

func (r *forkRepository) Get(context.Context, ID) (Meta, error) {
	return Meta{ID: "deadbeef", Title: "Source", Provider: "codex"}, nil
}

func (r *forkRepository) ReadEvents(context.Context, ID) ([]Event, error) {
	return append([]Event(nil), r.events...), nil
}

func (r *forkRepository) Create(_ context.Context, meta Meta) (Meta, error) {
	meta.ID = "fadecafe"
	return meta, nil
}

func (r *forkRepository) AppendEvent(ctx context.Context, _ ID, event Event) (Event, error) {
	r.suppressions = append(r.suppressions, EventNotificationsSuppressed(ctx))
	return event, nil
}

func TestForkMarksEveryCopiedEventAsNotificationSuppressed(t *testing.T) {
	repo := &forkRepository{events: []Event{
		{Seq: 1, Type: "tool_use_start", Name: "AskUserQuestion"},
		{Seq: 2, Type: "complete"},
		{Seq: 3, Type: "error", Message: "old failure"},
	}}
	service := New(repo, nil, nil, nil)

	if _, err := service.Fork(context.Background(), "deadbeef"); err != nil {
		t.Fatal(err)
	}
	if len(repo.suppressions) != len(repo.events) {
		t.Fatalf("copied %d events, want %d", len(repo.suppressions), len(repo.events))
	}
	for index, suppressed := range repo.suppressions {
		if !suppressed {
			t.Fatalf("copied event %d was not marked as notification-suppressed", index)
		}
	}
}
