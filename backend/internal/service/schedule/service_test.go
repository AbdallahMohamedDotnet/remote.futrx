package schedule

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

const (
	testChatID    servicechat.ID    = "abcd1234"
	testProjectID serviceproject.ID = "abcd1234"
	testOwner                       = "owner@example.com"
)

func TestCreateOnceValidatesTargetAndComputesNextRun(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := newTestService(repo, now)

	task, err := service.Create(context.Background(), CreateInput{
		Name:      " watch deploy ",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    " keep watching ",
		Kind:      KindOnce,
		At:        now.Add(time.Hour).UnixMilli(),
		Timezone:  "America/Toronto",
		MaxRuns:   12,
	}, " OWNER@EXAMPLE.COM ", false)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(task.ID) {
		t.Fatalf("generated invalid id %q", task.ID)
	}
	if task.OwnerEmail != testOwner || task.Name != "watch deploy" || task.Prompt != "keep watching" {
		t.Fatalf("task was not normalized: %#v", task)
	}
	if task.MaxRuns != 1 || task.Overlap != OverlapQueueOne {
		t.Fatalf("defaults not applied: %#v", task)
	}
	if task.NextRunAt != task.At || !task.Enabled || task.Status != StatusActive {
		t.Fatalf("invalid initial scheduling state: %#v", task)
	}
}

func TestCreateRejectsChatProjectMismatchAndAccessDenial(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	chats := &fakeChats{meta: servicechat.Meta{ID: testChatID, ProjectID: "ffff1111"}}
	access := &fakeAccess{allowed: true}
	identities := &fakeIdentities{registered: true}
	service := New(repo, chats, access, identities, &fakeExecutor{}, WithNow(func() time.Time { return now }))
	input := validOnceInput(now)

	if _, err := service.Create(context.Background(), input, testOwner, false); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}

	chats.meta.ProjectID = servicechat.ProjectID(testProjectID)
	access.allowed = false
	if _, err := service.Create(context.Background(), input, testOwner, false); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("access error = %v", err)
	}
}

func TestRunDuePersistsClaimAndCompletionDisablesTask(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(executor.starts) != 1 {
		t.Fatalf("starts = %d, want 1", len(executor.starts))
	}
	if !executor.claimObserved {
		t.Fatal("executor was called before the durable running claim was visible")
	}
	if !strings.Contains(executor.starts[0].prompt, `[scheduled task "watch deploy", fire 1/1]`) {
		t.Fatalf("unexpected generated prompt:\n%s", executor.starts[0].prompt)
	}

	executor.starts[0].handle.done <- RunResult{
		Output: "Deployment is healthy.\nSCHEDULE_STATUS=COMPLETE",
	}
	got := waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	if got.Enabled || got.Status != StatusCompleted || got.RunCount != 1 {
		t.Fatalf("unexpected completed task: %#v", got)
	}
	if got.LastStatus != RunStatusSucceeded || got.LastRunEnd == 0 {
		t.Fatalf("completion fields not recorded: %#v", got)
	}
}

func TestRecurringTaskQueuesOneOverlap(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), CreateInput{
		Name:      "watch deploy",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "Check deployment health.",
		Kind:      KindCron,
		Cron:      "* * * * *",
		Timezone:  "UTC",
		MaxRuns:   3,
	}, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Minute)
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(executor.starts) != 1 {
		t.Fatalf("overlap dispatched %d runs, want 1", len(executor.starts))
	}
	queued, _ := repo.Get(context.Background(), task.ID)
	if !queued.PendingRun || queued.RunCount != 1 {
		t.Fatalf("overlap was not coalesced: %#v", queued)
	}

	executor.starts[0].handle.done <- RunResult{Output: "Still deploying."}
	waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(executor.starts) != 2 {
		t.Fatalf("queued occurrence did not dispatch: starts = %d", len(executor.starts))
	}
	got, _ := repo.Get(context.Background(), task.ID)
	if got.PendingRun || got.RunCount != 2 || got.ActiveRunID == "" {
		t.Fatalf("invalid queued claim: %#v", got)
	}
}

func TestMaxRunsDisablesRecurringTask(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), CreateInput{
		Name:      "limited",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "Check once per minute.",
		Kind:      KindCron,
		Cron:      "* * * * *",
		Timezone:  "UTC",
		MaxRuns:   2,
	}, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	for run := 0; run < 2; run++ {
		now = now.Add(time.Minute)
		if err := service.RunDue(context.Background()); err != nil {
			t.Fatal(err)
		}
		executor.starts[run].handle.done <- RunResult{Output: "Not complete."}
		waitForTask(t, repo, task.ID, func(task Task) bool {
			return task.ActiveRunID == ""
		})
	}
	got, _ := repo.Get(context.Background(), task.ID)
	if got.Enabled || got.Status != StatusExhausted || got.RunCount != 2 {
		t.Fatalf("max-runs state = %#v", got)
	}
}

func TestRunDuePausesTaskWhenOwnerLosesAccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	access := &fakeAccess{allowed: true}
	executor := &fakeExecutor{repo: repo}
	service := New(
		repo,
		&fakeChats{meta: validChat()},
		access,
		&fakeIdentities{registered: true},
		executor,
		WithNow(func() time.Time { return now }),
	)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	access.allowed = false
	now = now.Add(2 * time.Hour)
	err = service.RunDue(context.Background())
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("RunDue error = %v", err)
	}
	got, _ := repo.Get(context.Background(), task.ID)
	if got.Enabled || got.Status != StatusError || !strings.Contains(got.LastError, "access denied") {
		t.Fatalf("revoked task was not disabled: %#v", got)
	}
	if len(executor.starts) != 0 {
		t.Fatal("unauthorized task was dispatched")
	}
}

func TestResourceMethodsRecheckCurrentProjectAccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	access := &fakeAccess{allowed: true}
	service := New(
		repo,
		&fakeChats{meta: validChat()},
		access,
		&fakeIdentities{registered: true},
		&fakeExecutor{repo: repo},
		WithNow(func() time.Time { return now }),
	)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	access.allowed = false

	if _, err := service.Get(context.Background(), task.ID, testOwner, false); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Get error = %v", err)
	}
	name := "changed"
	if _, err := service.Update(
		context.Background(), task.ID, UpdateInput{Name: &name}, testOwner, false,
	); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Update error = %v", err)
	}
	if err := service.Delete(context.Background(), task.ID, testOwner, false); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := service.Get(context.Background(), task.ID, "admin@example.com", true); err != nil {
		t.Fatalf("admin Get error = %v", err)
	}
}

func TestRunNowForcesPausedTaskWithoutChangingScheduleDefinition(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	enabled := false
	paused, err := service.Update(
		context.Background(),
		task.ID,
		UpdateInput{Enabled: &enabled},
		testOwner,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	claimed, err := service.RunNow(context.Background(), task.ID, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ActiveRunID == "" || claimed.Enabled || claimed.Status != StatusPaused {
		t.Fatalf("forced claim = %#v", claimed)
	}
	if claimed.NextRunAt != paused.NextRunAt || claimed.At != paused.At {
		t.Fatalf("RunNow changed the schedule definition: before=%#v after=%#v", paused, claimed)
	}
	executor.starts[0].handle.done <- RunResult{Output: "Still pending."}
	got := waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	if got.Enabled || got.Status != StatusPaused || got.NextRunAt != paused.NextRunAt {
		t.Fatalf("forced completion changed paused schedule: %#v", got)
	}
	service.Close()
}

func TestRunNowReturnsQueuedStateWhenExecutorIsBusy(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo, startErr: ErrExecutorBusy}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	queued, err := service.RunNow(context.Background(), task.ID, testOwner, false)
	if err != nil {
		t.Fatalf("RunNow returned an error for an accepted queue-one run: %v", err)
	}
	if queued.ActiveRunID != "" || !queued.PendingRun || !queued.PendingRunForced {
		t.Fatalf("RunNow queued state = %#v", queued)
	}
	if queued.RunCount != 0 || queued.LastStatus != RunStatusQueued {
		t.Fatalf("busy rejection consumed a run: %#v", queued)
	}
}

func TestExecutorBusyQueueCoalescesMissedCronDeadlines(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo, startErr: ErrExecutorBusy}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), CreateInput{
		Name:      "watch deploy",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "Check deployment health.",
		Kind:      KindCron,
		Cron:      "* * * * *",
		Timezone:  "UTC",
	}, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Minute)
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatalf("busy queue should be accepted: %v", err)
	}
	queued, _ := repo.Get(context.Background(), task.ID)
	if !queued.PendingRun || queued.NextRunAt != now.Add(time.Minute).UnixMilli() {
		t.Fatalf("initial queued state = %#v", queued)
	}

	now = now.Add(4 * time.Minute)
	executor.mu.Lock()
	executor.startErr = nil
	executor.mu.Unlock()
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, _ := repo.Get(context.Background(), task.ID)
	if running.PendingRun {
		t.Fatalf("accepted queued run remained pending: %#v", running)
	}
	wantNext := now.Add(time.Minute).UnixMilli()
	if running.NextRunAt != wantNext {
		t.Fatalf("next run = %d, want %d after coalescing missed deadlines", running.NextRunAt, wantNext)
	}

	// The stale cron deadline must not queue another immediate follow-up while
	// the accepted coalesced run is still active.
	if err := service.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(context.Background(), task.ID)
	if got.PendingRun {
		t.Fatalf("missed deadlines produced a second follow-up: %#v", got)
	}
	if len(executor.starts) != 1 {
		t.Fatalf("executor starts = %d, want 1", len(executor.starts))
	}
	executor.starts[0].handle.done <- RunResult{Output: "still pending"}
	waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	service.Close()
}

func TestOnceRunUsesCompletedOrErrorTerminalStatus(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		result     RunResult
		wantStatus Status
		wantRun    RunStatus
	}{
		{
			name:       "success without completion marker",
			result:     RunResult{Output: "one-time work finished"},
			wantStatus: StatusCompleted,
			wantRun:    RunStatusSucceeded,
		},
		{
			name:       "failure",
			result:     RunResult{Err: errors.New("agent failed")},
			wantStatus: StatusError,
			wantRun:    RunStatusFailed,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
			repo := newMemoryRepository()
			executor := &fakeExecutor{repo: repo}
			service := newTestServiceWithExecutor(repo, &now, executor)
			task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
			if err != nil {
				t.Fatal(err)
			}

			now = now.Add(2 * time.Hour)
			if err := service.RunDue(context.Background()); err != nil {
				t.Fatal(err)
			}
			executor.starts[0].handle.done <- testCase.result
			got := waitForTask(t, repo, task.ID, func(task Task) bool {
				return task.ActiveRunID == ""
			})
			if got.Enabled || got.Status != testCase.wantStatus || got.LastStatus != testCase.wantRun {
				t.Fatalf("once terminal state = %#v", got)
			}
			service.Close()
		})
	}
}

func TestCompleteClaimIsScopedAndPreservedAfterRunFinishes(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := service.RunNow(context.Background(), task.ID, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteClaim(context.Background(), task.ID, "wrong"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("wrong claim error = %v", err)
	}
	completed, err := service.CompleteClaim(context.Background(), task.ID, claimed.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Enabled || completed.Status != StatusCompleted {
		t.Fatalf("CompleteClaim result = %#v", completed)
	}

	executor.starts[0].handle.done <- RunResult{Output: "final output without marker"}
	got := waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	if got.Status != StatusCompleted || got.Enabled {
		t.Fatalf("executor finish overwrote explicit completion: %#v", got)
	}
	service.Close()
}

func TestRunNowCompletionSurvivesRequestCancellation(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := newTestServiceWithExecutor(repo, &now, executor)
	task, err := service.Create(context.Background(), validOnceInput(now), testOwner, false)
	if err != nil {
		t.Fatal(err)
	}

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	if _, err := service.RunNow(requestCtx, task.ID, testOwner, false); err != nil {
		t.Fatal(err)
	}
	cancelRequest()
	executor.starts[0].handle.done <- RunResult{Output: "done\nSCHEDULE_STATUS=COMPLETE"}
	got := waitForTask(t, repo, task.ID, func(task Task) bool {
		return task.ActiveRunID == ""
	})
	if got.Status != StatusCompleted {
		t.Fatalf("request cancellation abandoned completion: %#v", got)
	}
	service.Close()
}

func TestStartRecoversAbandonedClaim(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	task := Task{
		ID:               "0123456789abcdef01234567",
		Name:             "recover",
		OwnerEmail:       testOwner,
		ProjectID:        testProjectID,
		ChatID:           testChatID,
		Prompt:           "continue",
		Kind:             KindCron,
		Cron:             "0 0 * * *",
		Timezone:         "UTC",
		Enabled:          true,
		Status:           StatusRunning,
		NextRunAt:        now.Add(12 * time.Hour).UnixMilli(),
		RunCount:         1,
		ActiveRunID:      "dead-run",
		ActiveRunStarted: now.Add(-time.Minute).UnixMilli(),
	}
	if _, err := repo.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	service := newTestService(repo, now)
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	service.Close()
	got, _ := repo.Get(context.Background(), task.ID)
	if got.ActiveRunID != "" || got.LastStatus != RunStatusAbandoned || got.Status != StatusActive {
		t.Fatalf("claim was not recovered: %#v", got)
	}
}

func TestSchedulerLoopWakesForNewTask(t *testing.T) {
	repo := newMemoryRepository()
	executor := &fakeExecutor{repo: repo}
	service := New(
		repo,
		&fakeChats{meta: validChat()},
		&fakeAccess{allowed: true},
		&fakeIdentities{registered: true},
		executor,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	_, err := service.Create(context.Background(), CreateInput{
		Name:      "soon",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "run soon",
		Kind:      KindOnce,
		At:        time.Now().Add(100 * time.Millisecond).UnixMilli(),
		Timezone:  "UTC",
	}, testOwner, false)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		executor.mu.Lock()
		started := len(executor.starts) == 1
		executor.mu.Unlock()
		if started {
			return
		}
		select {
		case <-deadline:
			t.Fatal("scheduler did not wake and dispatch the new task")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestDelayUntilNextSaturatesFarFutureDeadline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	if _, err := repo.Create(context.Background(), Task{
		ID:        "0123456789abcdef01234567",
		Enabled:   true,
		NextRunAt: int64(1<<63 - 1),
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(repo, now)

	delay, ok, err := service.delayUntilNext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok || delay <= 0 {
		t.Fatalf("far-future delay = %v, ok=%t; want a positive saturated duration", delay, ok)
	}
}

func TestSchedulerBacksOffAfterDurableUpdateFailure(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	base := newMemoryRepository()
	if _, err := base.Create(context.Background(), Task{
		ID:         "0123456789abcdef01234567",
		Name:       "due",
		OwnerEmail: testOwner,
		ProjectID:  testProjectID,
		ChatID:     testChatID,
		Prompt:     "continue",
		Kind:       KindCron,
		Cron:       "* * * * *",
		Timezone:   "UTC",
		Enabled:    true,
		Status:     StatusActive,
		NextRunAt:  now.Add(-time.Minute).UnixMilli(),
		Overlap:    OverlapQueueOne,
	}); err != nil {
		t.Fatal(err)
	}
	repo := &failingUpdateRepository{
		memoryRepository: base,
		attempted:        make(chan struct{}, 8),
		err:              errors.New("disk full"),
	}
	service := New(
		repo,
		&fakeChats{meta: validChat()},
		&fakeAccess{allowed: true},
		&fakeIdentities{registered: true},
		&fakeExecutor{repo: base},
		WithNow(func() time.Time { return now }),
		WithBusyRetry(200*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	select {
	case <-repo.attempted:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not attempt the due durable update")
	}
	select {
	case <-repo.attempted:
		t.Fatal("scheduler retried a failed durable update without backoff")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCompletionMarkerMustBeFinalLine(t *testing.T) {
	t.Parallel()
	if !HasCompletionMarker("done\nSCHEDULE_STATUS=COMPLETE") {
		t.Fatal("exact final marker was not recognized")
	}
	if !HasCompletionMarker("done\nTASK_COMPLETE") {
		t.Fatal("legacy final marker was not recognized")
	}
	if HasCompletionMarker("SCHEDULE_STATUS=COMPLETE\nbut actually not done") {
		t.Fatal("non-final marker was accepted")
	}
}

type memoryRepository struct {
	mu    sync.Mutex
	tasks map[ID]Task
}

type failingUpdateRepository struct {
	*memoryRepository
	attempted chan struct{}
	err       error
}

func (r *failingUpdateRepository) Update(
	context.Context,
	ID,
	func(*Task) error,
) (Task, error) {
	select {
	case r.attempted <- struct{}{}:
	default:
	}
	return Task{}, r.err
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{tasks: make(map[ID]Task)}
}

func (r *memoryRepository) List(ctx context.Context) ([]Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		out = append(out, task)
	}
	return out, nil
}

func (r *memoryRepository) Create(ctx context.Context, task Task) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[task.ID]; ok {
		return Task{}, ErrAlreadyExists
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *memoryRepository) Get(ctx context.Context, id ID) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return task, nil
}

func (r *memoryRepository) Update(ctx context.Context, id ID, fn func(*Task) error) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	if err := fn(&task); err != nil {
		return Task{}, err
	}
	r.tasks[id] = task
	return task, nil
}

func (r *memoryRepository) Delete(ctx context.Context, id ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}

type fakeChats struct {
	meta servicechat.Meta
	err  error
}

func (c *fakeChats) Get(ctx context.Context, id servicechat.ID) (servicechat.Meta, error) {
	if c.err != nil {
		return servicechat.Meta{}, c.err
	}
	if id != c.meta.ID {
		return servicechat.Meta{}, servicechat.ErrNotFound
	}
	return c.meta, nil
}

type fakeAccess struct {
	mu      sync.Mutex
	allowed bool
	err     error
}

func (a *fakeAccess) HasAccess(ctx context.Context, id serviceproject.ID, email string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allowed, a.err
}

type fakeIdentities struct {
	registered bool
	admin      bool
	err        error
}

func (i *fakeIdentities) IsRegistered(ctx context.Context, email string) (bool, error) {
	return i.registered, i.err
}

func (i *fakeIdentities) IsAdmin(ctx context.Context, email string) (bool, error) {
	return i.admin, i.err
}

type fakeRunHandle struct {
	done chan RunResult
}

func (h *fakeRunHandle) Done() <-chan RunResult { return h.done }

type fakeStart struct {
	chatID servicechat.ID
	prompt string
	handle *fakeRunHandle
}

type fakeExecutor struct {
	mu            sync.Mutex
	repo          *memoryRepository
	starts        []fakeStart
	startErr      error
	claimObserved bool
}

func (e *fakeExecutor) StartScheduledPrompt(
	ctx context.Context,
	task Task,
	prompt string,
) (RunHandle, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.startErr != nil {
		return nil, e.startErr
	}
	if e.repo != nil {
		tasks, _ := e.repo.List(context.Background())
		e.claimObserved = len(tasks) == 1 &&
			tasks[0].ActiveRunID != "" &&
			tasks[0].LastStatus == RunStatusRunning
	}
	handle := &fakeRunHandle{done: make(chan RunResult, 1)}
	e.starts = append(e.starts, fakeStart{chatID: task.ChatID, prompt: prompt, handle: handle})
	return handle, nil
}

func validChat() servicechat.Meta {
	return servicechat.Meta{ID: testChatID, ProjectID: servicechat.ProjectID(testProjectID)}
}

func validOnceInput(now time.Time) CreateInput {
	return CreateInput{
		Name:      "watch deploy",
		ProjectID: testProjectID,
		ChatID:    testChatID,
		Prompt:    "Check deployment health.",
		Kind:      KindOnce,
		At:        now.Add(time.Hour).UnixMilli(),
		Timezone:  "UTC",
	}
}

func newTestService(repo *memoryRepository, now time.Time) *Service {
	return New(
		repo,
		&fakeChats{meta: validChat()},
		&fakeAccess{allowed: true},
		&fakeIdentities{registered: true},
		&fakeExecutor{repo: repo},
		WithNow(func() time.Time { return now }),
	)
}

func newTestServiceWithExecutor(
	repo *memoryRepository,
	now *time.Time,
	executor *fakeExecutor,
) *Service {
	return New(
		repo,
		&fakeChats{meta: validChat()},
		&fakeAccess{allowed: true},
		&fakeIdentities{registered: true},
		executor,
		WithNow(func() time.Time { return *now }),
	)
}

func waitForTask(
	t *testing.T,
	repo *memoryRepository,
	id ID,
	condition func(Task) bool,
) Task {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		task, err := repo.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if condition(task) {
			return task
		}
		time.Sleep(time.Millisecond)
	}
	task, _ := repo.Get(context.Background(), id)
	t.Fatalf("condition not reached for task: %#v", task)
	return Task{}
}
