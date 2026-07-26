package schedule

import (
	"context"
	"errors"
	"testing"
	"time"
)

func guardedService(repo *memoryRepository, now time.Time, options ...Option) *Service {
	base := []Option{WithNow(func() time.Time { return now })}
	return New(
		repo,
		&fakeChats{meta: validChat()},
		&fakeAccess{allowed: true},
		&fakeIdentities{registered: true},
		&fakeExecutor{repo: repo},
		append(base, options...)...,
	)
}

func cronCreateInput() CreateInput {
	return CreateInput{
		Name:      "watch",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "check things",
		Kind:      KindCron,
		Cron:      "*/10 * * * *",
		Timezone:  "UTC",
	}
}

func TestCreateRejectsCronBelowMinInterval(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	service := guardedService(newMemoryRepository(), now, WithMinInterval(5*time.Minute))

	input := cronCreateInput()
	input.Cron = "* * * * *"
	if _, err := service.Create(context.Background(), input, testOwner, false); !errors.Is(err, ErrIntervalTooSmall) {
		t.Fatalf("Create(* * * * *) error = %v, want ErrIntervalTooSmall", err)
	}

	input.Cron = "*/5 * * * *"
	if _, err := service.Create(context.Background(), input, testOwner, false); err != nil {
		t.Fatalf("Create(*/5) error = %v, want nil", err)
	}
}

func TestClaimDefersFiresBelowFloor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := guardedService(repo, now, WithMinInterval(10*time.Minute))

	lastStart := now.Add(-2 * time.Minute)
	seed := Task{
		ID: newID(), Name: "hot", OwnerEmail: testOwner,
		ProjectID: testProjectID, ChatID: testChatID,
		Prompt: "p", Kind: KindCron, Cron: "*/5 * * * *", Timezone: "UTC",
		Enabled: true, Status: StatusActive, Overlap: OverlapQueueOne,
		NextRunAt: now.UnixMilli(), LastRunAt: lastStart.UnixMilli(),
	}
	if _, err := repo.Create(context.Background(), seed); err != nil {
		t.Fatal(err)
	}

	_, claimed, err := service.claim(context.Background(), seed.ID, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("claim succeeded below the interval floor")
	}
	task, err := repo.Get(context.Background(), seed.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantEarliest := lastStart.Add(10 * time.Minute).UnixMilli()
	if task.NextRunAt != wantEarliest {
		t.Fatalf("NextRunAt = %d, want deferred to %d", task.NextRunAt, wantEarliest)
	}
	if task.ActiveRunID != "" || task.RunCount != 0 {
		t.Fatalf("deferral must not consume the occurrence: %#v", task)
	}
}

func TestClaimHonorsGlobalConcurrencyCap(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := guardedService(repo, now, WithMaxConcurrentRuns(1))

	running := Task{
		ID: newID(), Name: "busy", OwnerEmail: testOwner,
		ProjectID: testProjectID, ChatID: "aaaa0000aaaa0000aaaa0000", Prompt: "p",
		Kind: KindCron, Cron: "*/10 * * * *", Timezone: "UTC",
		Enabled: true, Status: StatusRunning, Overlap: OverlapQueueOne,
		ActiveRunID: "run-1", RunCount: 1,
	}
	queuedTask := Task{
		ID: newID(), Name: "waiting", OwnerEmail: testOwner,
		ProjectID: testProjectID, ChatID: testChatID, Prompt: "p",
		Kind: KindCron, Cron: "*/10 * * * *", Timezone: "UTC",
		Enabled: true, Status: StatusActive, Overlap: OverlapQueueOne,
		NextRunAt: now.UnixMilli(),
	}
	for _, task := range []Task{running, queuedTask} {
		if _, err := repo.Create(context.Background(), task); err != nil {
			t.Fatal(err)
		}
	}

	_, claimed, err := service.claim(context.Background(), queuedTask.ID, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("claim succeeded past the concurrency cap")
	}
	task, err := repo.Get(context.Background(), queuedTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !task.PendingRun || task.LastStatus != RunStatusQueued {
		t.Fatalf("saturated queue_one task must queue: %#v", task)
	}
	if task.ActiveRunID != "" {
		t.Fatalf("saturated task must not claim: %#v", task)
	}
}

func TestCreateEnforcesProjectQuota(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := guardedService(repo, now, WithMaxTasksPerProject(1))

	if _, err := service.Create(context.Background(), cronCreateInput(), testOwner, false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := service.Create(context.Background(), cronCreateInput(), testOwner, false); !errors.Is(err, ErrProjectQuota) {
		t.Fatalf("second create error = %v, want ErrProjectQuota", err)
	}

	// Terminal tasks are history, not quota. Complete the standing task and
	// creation opens up again.
	tasks, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), tasks[0].ID, testOwner, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), cronCreateInput(), testOwner, false); err != nil {
		t.Fatalf("create after completing prior task: %v", err)
	}
}

func TestAgentCreatedTasksStartDisabledUntilArmed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := guardedService(repo, now)

	input := cronCreateInput()
	input.CreatedByAgent = true
	task, err := service.Create(context.Background(), input, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	if task.Enabled || task.Status != StatusPaused || !task.CreatedByAgent {
		t.Fatalf("agent-created task must start parked: %#v", task)
	}
	if task.NextRunAt != 0 {
		t.Fatalf("parked task must not be on the clock: %#v", task)
	}

	// The user arming it puts it on the clock.
	enabled := true
	armed, err := service.Update(context.Background(), task.ID, UpdateInput{Enabled: &enabled}, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	if !armed.Enabled || armed.Status != StatusActive || armed.NextRunAt == 0 {
		t.Fatalf("armed task must be scheduled: %#v", armed)
	}
}
