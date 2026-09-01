package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	"github.com/futrx-com/remote.futrx.com/internal/service/runhub"
	"github.com/futrx-com/remote.futrx.com/internal/stores/filechat"
)

type countingPromptProvider struct {
	mu   sync.Mutex
	runs int
	id   agent.ProviderID
}

type failNthUpdateRepository struct {
	servicechat.Repository
	mu     sync.Mutex
	calls  int
	failAt int
}

func (repository *failNthUpdateRepository) Update(
	ctx context.Context,
	id servicechat.ID,
	change func(*servicechat.Meta),
) (servicechat.Meta, error) {
	repository.mu.Lock()
	repository.calls++
	call := repository.calls
	repository.mu.Unlock()
	if call == repository.failAt {
		return servicechat.Meta{}, errors.New("injected metadata write failure")
	}
	return repository.Repository.Update(ctx, id, change)
}

func (repository *failNthUpdateRepository) failNextUpdate() {
	repository.mu.Lock()
	repository.failAt = repository.calls + 1
	repository.mu.Unlock()
}

func (provider *countingPromptProvider) ID() agent.ProviderID { return provider.id }
func (provider *countingPromptProvider) Capabilities(context.Context, agent.CapabilityRequest) (agent.Capabilities, error) {
	return agent.Capabilities{Provider: provider.id}, nil
}
func (provider *countingPromptProvider) Run(context.Context, agent.RunRequest, func(agent.Event)) error {
	provider.mu.Lock()
	provider.runs++
	provider.mu.Unlock()
	return nil
}
func (provider *countingPromptProvider) runCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.runs
}

func TestStartRejectsStalePlanSynchronouslyWithoutChangingPreference(t *testing.T) {
	ctx := context.Background()
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Create(ctx, servicechat.Meta{
		ID:       "abcdef123456",
		Provider: servicechat.ProviderClaude,
		Mode:     string(agent.RunModePlan),
	})
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, nil, nil, runhub.New(store), agent.NewRegistry())
	var transients []ChatEvent
	handle, err := service.Start(StartInput{ChatID: meta.ID, Prompt: "do not mutate"}, func(event ChatEvent) {
		transients = append(transients, event)
	})
	if !errors.Is(err, agent.ErrUnsupportedRunMode) {
		t.Fatalf("start error = %v", err)
	}
	if handle.ID != 0 || handle.Done != nil {
		t.Fatalf("rejected run returned a handle: %#v", handle)
	}
	if len(transients) != 1 || transients[0].Type != "error" ||
		transients[0].Message != unsupportedRunModeMessage {
		t.Fatalf("transient events = %#v", transients)
	}
	updated, err := store.Get(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Mode != string(agent.RunModePlan) {
		t.Fatalf("saved mode = %q", updated.Mode)
	}
	events, err := store.ReadEvents(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("rejected prompt was persisted: %#v", events)
	}

	// A recurring scheduled task must fail closed again on its next fire. The
	// first rejection must not rewrite Plan and turn a future occurrence into a
	// Default execution.
	if _, err := service.Start(StartInput{ChatID: meta.ID, Prompt: "still do not mutate"}, nil); !errors.Is(err, agent.ErrUnsupportedRunMode) {
		t.Fatalf("second start error = %v", err)
	}
}

func TestStartRejectsStaleClientExecutionPreferences(t *testing.T) {
	ctx := context.Background()
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Create(ctx, servicechat.Meta{
		ID:       "abcdef654321",
		Provider: servicechat.ProviderClaude,
		Mode:     string(agent.RunModeDefault),
	})
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, nil, nil, runhub.New(store), agent.NewRegistry())
	var transients []ChatEvent
	_, err = service.Start(StartInput{
		ChatID: meta.ID,
		Prompt: "use the controls I saw",
		Expected: &ExecutionPreferences{
			Provider: servicechat.ProviderCodex,
			Mode:     string(agent.RunModeDefault),
		},
	}, func(event ChatEvent) { transients = append(transients, event) })
	if !errors.Is(err, ErrExecutionPreferencesChanged) {
		t.Fatalf("start error = %v", err)
	}
	if len(transients) != 1 || transients[0].Message != executionPreferencesChangedMessage {
		t.Fatalf("transients = %#v", transients)
	}
	events, err := store.ReadEvents(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("stale prompt was persisted: %#v", events)
	}
}

func TestStartDeduplicatesPersistedInteractiveClientID(t *testing.T) {
	ctx := context.Background()
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Create(ctx, servicechat.Meta{
		ID:       "feedface1234",
		Provider: servicechat.ProviderCodex,
		Mode:     string(agent.RunModeDefault),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &countingPromptProvider{id: agent.ProviderCodex}
	registry := agent.NewRegistry()
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}
	service := New(store, nil, nil, runhub.New(store), registry)

	first, err := service.Start(StartInput{
		ChatID:   meta.ID,
		Prompt:   "make the change",
		ClientID: "queue-1",
		Expected: &ExecutionPreferences{
			Provider: servicechat.ProviderCodex,
			Mode:     string(agent.RunModeDefault),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-first.Done; result.Err != nil {
		t.Fatal(result.Err)
	}

	// A reconnect can resend after controls changed and before its transient ack
	// was observed. The durable client ID wins over the now-stale expectation:
	// acknowledge the original acceptance without another provider run.
	duplicate, err := service.Start(StartInput{
		ChatID:   meta.ID,
		Prompt:   "make the change",
		ClientID: "queue-1",
		Expected: &ExecutionPreferences{
			Provider: servicechat.ProviderClaude,
			Mode:     string(agent.RunModeDefault),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-duplicate.Done; result.Err != nil {
		t.Fatal(result.Err)
	}
	if _, err := service.Start(StartInput{
		ChatID:   meta.ID,
		Prompt:   "different change",
		ClientID: "queue-1",
	}, nil); !errors.Is(err, ErrPromptClientIDConflict) {
		t.Fatalf("client id collision error = %v", err)
	}
	if got := provider.runCount(); got != 1 {
		t.Fatalf("provider runs = %d, want 1", got)
	}
	events, err := store.ReadEvents(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "user" {
		t.Fatalf("events = %#v", events)
	}
	var data struct {
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(events[0].Data, &data); err != nil || data.ClientID != "queue-1" {
		t.Fatalf("accepted client id = %q, err = %v", data.ClientID, err)
	}
}

func TestStartDeduplicatesInteractiveClientIDAfterVisibleHistoryRewind(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := filechat.New(root)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Create(ctx, servicechat.Meta{
		ID:       "facefeed1234",
		Provider: servicechat.ProviderCodex,
		Mode:     string(agent.RunModeDefault),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &countingPromptProvider{id: agent.ProviderCodex}
	registry := agent.NewRegistry()
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}
	service := New(store, nil, nil, runhub.New(store), registry)
	preferences := &ExecutionPreferences{
		Provider: servicechat.ProviderCodex,
		Mode:     string(agent.RunModeDefault),
	}

	first, err := service.Start(StartInput{
		ChatID: meta.ID, Prompt: "do this once", ClientID: "rewound-client", Expected: preferences,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-first.Done; result.Err != nil {
		t.Fatal(result.Err)
	}
	events, err := store.ReadEvents(ctx, meta.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("accepted events = %#v, err = %v", events, err)
	}
	if _, err := store.TruncateEventsBefore(ctx, meta.ID, events[0].T); err != nil {
		t.Fatal(err)
	}
	if remaining, err := store.ReadEvents(ctx, meta.ID); err != nil || len(remaining) != 0 {
		t.Fatalf("rewound events = %#v, err = %v", remaining, err)
	}
	reopened, err := filechat.New(root)
	if err != nil {
		t.Fatal(err)
	}
	restartedService := New(reopened, nil, nil, runhub.New(reopened), registry)

	duplicate, err := restartedService.Start(StartInput{
		ChatID: meta.ID, Prompt: "do this once", ClientID: "rewound-client", Expected: preferences,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-duplicate.Done; result.Err != nil {
		t.Fatal(result.Err)
	}
	if _, err := restartedService.Start(StartInput{
		ChatID: meta.ID, Prompt: "do something else", ClientID: "rewound-client", Expected: preferences,
	}, nil); !errors.Is(err, ErrPromptClientIDConflict) {
		t.Fatalf("client id collision after rewind = %v", err)
	}
	if got := provider.runCount(); got != 1 {
		t.Fatalf("provider runs = %d, want 1", got)
	}

	storedMeta, err := reopened.Get(ctx, meta.ID)
	if err != nil || storedMeta.PromptReceipts.Len() != 1 {
		t.Fatalf("prompt receipts = %#v, err = %v", storedMeta.PromptReceipts, err)
	}
	publicJSON, err := json.Marshal(storedMeta)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "promptReceipts") || strings.Contains(string(publicJSON), "rewound-client") {
		t.Fatalf("internal prompt receipts leaked through API JSON: %s", publicJSON)
	}
}

func TestStartTreatsCommitWriteFailureAfterUserEventAsAccepted(t *testing.T) {
	ctx := context.Background()
	underlying, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta, err := underlying.Create(ctx, servicechat.Meta{
		ID:       "decafbad1234",
		Title:    "Already titled",
		Provider: servicechat.ProviderCodex,
		Mode:     string(agent.RunModeDefault),
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := &failNthUpdateRepository{Repository: underlying, failAt: 1}
	provider := &countingPromptProvider{id: agent.ProviderCodex}
	registry := agent.NewRegistry()
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}
	hub := runhub.New(repository)
	service := New(repository, nil, nil, hub, registry)
	chatService := servicechat.New(repository, nil, nil, hub)
	input := StartInput{
		ChatID: meta.ID, Prompt: "perform one action", ClientID: "indeterminate-client",
		Expected: &ExecutionPreferences{
			Provider: servicechat.ProviderCodex,
			Mode:     string(agent.RunModeDefault),
		},
	}

	first, err := service.Start(input, nil)
	if err != nil {
		t.Fatalf("durable user event must be accepted despite receipt commit failure: %v", err)
	}
	if result := <-first.Done; result.Err != nil {
		t.Fatal(result.Err)
	}
	if got := provider.runCount(); got != 1 {
		t.Fatalf("provider runs after acceptance = %d, want 1", got)
	}

	stored, err := underlying.Get(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PromptReceipts.Len() != 0 {
		t.Fatalf("failed receipt write unexpectedly persisted: %#v", stored.PromptReceipts)
	}
	events, err := underlying.ReadEvents(ctx, meta.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("accepted events = %#v, err = %v", events, err)
	}
	repository.failNextUpdate()
	if _, err := chatService.Rewind(ctx, meta.ID, events[0].T); err == nil {
		t.Fatal("rewind succeeded without persisting the accepted receipt")
	}
	if remaining, err := underlying.ReadEvents(ctx, meta.ID); err != nil || len(remaining) != 1 {
		t.Fatalf("failed rewind changed events = %#v, err = %v", remaining, err)
	}
	if _, err := chatService.Rewind(ctx, meta.ID, events[0].T); err != nil {
		t.Fatalf("rewind must backfill the accepted receipt: %v", err)
	}
	if remaining, err := underlying.ReadEvents(ctx, meta.ID); err != nil || len(remaining) != 0 {
		t.Fatalf("rewound events = %#v, err = %v", remaining, err)
	}

	duplicate, err := service.Start(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result := <-duplicate.Done; result.Err != nil {
		t.Fatal(result.Err)
	}
	if got := provider.runCount(); got != 1 {
		t.Fatalf("provider reran accepted prompt after receipt backfill: %d", got)
	}
	stored, err = underlying.Get(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PromptReceipts.Len() != 1 {
		t.Fatalf("rewind did not persist accepted receipt: %#v", stored.PromptReceipts)
	}
}
