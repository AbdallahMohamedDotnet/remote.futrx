package schedulecapability

import (
	"context"
	"errors"
	"testing"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
	serviceprompt "github.com/futrx-com/remote.futrx.com/internal/service/prompt"
)

func TestRegistryIssuesScopedRevocableCapabilities(t *testing.T) {
	registry := New("https://remote.example.com/")
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	registry.now = func() time.Time { return now }

	access, err := registry.IssueScheduleTool(context.Background(), serviceprompt.ScheduleToolRequest{
		Actor:     serviceprompt.Actor{Email: " User@Example.com ", IsAdmin: true},
		ChatID:    servicechat.ID("abcdef123456"),
		ProjectID: serviceproject.ID("abcdef654321"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if access.APIURL != "https://remote.example.com/agent-api/schedules" {
		t.Fatalf("APIURL = %q", access.APIURL)
	}
	grant, err := registry.Resolve(access.Token)
	if err != nil {
		t.Fatal(err)
	}
	if grant.OwnerEmail != "user@example.com" || grant.Scope != ScopeManage || !grant.IsAdmin {
		t.Fatalf("grant = %#v", grant)
	}
	access.Revoke()
	if _, err := registry.Resolve(access.Token); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("Resolve after revoke error = %v", err)
	}
}

func TestScheduledRunCapabilityCanOnlyCompleteItselfAndExpires(t *testing.T) {
	registry := New("https://remote.example.com")
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	registry.now = func() time.Time { return now }

	access, err := registry.IssueScheduleTool(context.Background(), serviceprompt.ScheduleToolRequest{
		Actor:           serviceprompt.Actor{Email: "user@example.com"},
		ChatID:          "abcdef123456",
		ProjectID:       "abcdef654321",
		ScheduledTaskID: "task-1",
		ScheduledRunID:  "run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := registry.Resolve(access.Token)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Scope != ScopeCompleteSelf ||
		grant.ScheduledTaskID != "task-1" ||
		grant.ScheduledRunID != "run-1" {
		t.Fatalf("grant = %#v", grant)
	}
	now = now.Add(5 * time.Hour)
	if _, err := registry.Resolve(access.Token); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("Resolve expired error = %v", err)
	}
}

func TestScheduledRunCapabilityRequiresTaskAndRunPair(t *testing.T) {
	t.Parallel()
	registry := New("https://remote.example.com")
	base := serviceprompt.ScheduleToolRequest{
		Actor:     serviceprompt.Actor{Email: "user@example.com"},
		ChatID:    "abcdef123456",
		ProjectID: "abcdef654321",
	}

	taskOnly := base
	taskOnly.ScheduledTaskID = "task-1"
	if _, err := registry.IssueScheduleTool(context.Background(), taskOnly); err == nil {
		t.Fatal("task-only completion capability was accepted")
	}

	runOnly := base
	runOnly.ScheduledRunID = "run-1"
	if _, err := registry.IssueScheduleTool(context.Background(), runOnly); err == nil {
		t.Fatal("run-only completion capability was accepted")
	}
}
