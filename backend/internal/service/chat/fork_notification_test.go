package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/service/agent/module"
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

type rejectsIdleChangeRunController struct{}

func (*rejectsIdleChangeRunController) IsRunning(ID) bool { return false }
func (*rejectsIdleChangeRunController) WhileIdle(ID, func() error) (bool, error) {
	return false, nil
}

func (*rejectsIdleChangeRunController) CancelAndWhileIdle(context.Context, ID, func() error) error {
	return ErrChatRunning
}
func (*rejectsIdleChangeRunController) Cancel(context.Context, ID) error { return nil }

type forkProviderPolicy map[string]bool

func (p forkProviderPolicy) HasProvider(provider string) bool { return p[provider] }
func (p forkProviderPolicy) SupportsScope(provider string, _ agentmodule.ExecutionScope) bool {
	return p[provider]
}
func (p forkProviderPolicy) SupportsRunMode(provider string, mode agent.RunMode) bool {
	return p[provider] && mode == agent.RunModeDefault
}

type scopedProviderPolicy map[string]map[agentmodule.ExecutionScope]bool

func (p scopedProviderPolicy) HasProvider(provider string) bool { return p[provider] != nil }
func (p scopedProviderPolicy) SupportsScope(provider string, scope agentmodule.ExecutionScope) bool {
	return p[provider][scope]
}
func (p scopedProviderPolicy) SupportsRunMode(provider string, mode agent.RunMode) bool {
	return p[provider] != nil && mode == agent.RunModeDefault
}

type defaultScopedProviderPolicy struct {
	scopedProviderPolicy
	provider Provider
}

func (p defaultScopedProviderPolicy) DefaultProvider(agentmodule.ExecutionScope) Provider {
	return p.provider
}

func TestCreateUsesConfiguredProviderCatalog(t *testing.T) {
	repo := &forkRepository{}
	service := New(
		repo,
		nil,
		nil,
		nil,
		WithProviderPolicy(forkProviderPolicy{"future-agent": true}),
	)
	created, err := service.Create(context.Background(), CreateInput{Provider: "future-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Provider != "future-agent" {
		t.Fatalf("created provider = %q", created.Provider)
	}
	if _, err := service.Create(context.Background(), CreateInput{Provider: ProviderCodex}); err != ErrInvalidProvider {
		t.Fatalf("unconfigured provider error = %v, want ErrInvalidProvider", err)
	}
}

func TestCreateUsesCatalogDefaultAndRejectsUnsafeExplicitProvider(t *testing.T) {
	repo := &forkRepository{}
	policy := defaultScopedProviderPolicy{
		scopedProviderPolicy: scopedProviderPolicy{
			"future-agent": {agentmodule.ScopeHost: true},
		},
		provider: "future-agent",
	}
	service := New(repo, nil, nil, nil, WithProviderPolicy(policy))
	created, err := service.Create(context.Background(), CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	if created.Provider != "future-agent" {
		t.Fatalf("default provider = %q, want future-agent", created.Provider)
	}
	if _, err := service.Create(context.Background(), CreateInput{Provider: "bad provider"}); err != ErrInvalidProvider {
		t.Fatalf("unsafe provider error = %v, want ErrInvalidProvider", err)
	}
}

func TestCreateAndUpdateEnforceProviderExecutionScope(t *testing.T) {
	policy := scopedProviderPolicy{
		"host-agent":    {agentmodule.ScopeHost: true},
		"project-agent": {agentmodule.ScopeProject: true},
	}
	service := New(&forkRepository{}, nil, nil, nil, WithProviderPolicy(policy))
	if _, err := service.Create(context.Background(), CreateInput{Provider: "host-agent", ProjectID: "abcd"}); err != ErrInvalidProvider {
		t.Fatalf("project chat with host agent error = %v", err)
	}
	if _, err := service.Create(context.Background(), CreateInput{Provider: "project-agent"}); err != ErrInvalidProvider {
		t.Fatalf("host chat with project agent error = %v", err)
	}

	repo := &forkRepository{source: Meta{ID: "deadbeef", Provider: "project-agent", ProjectID: "abcd"}}
	service = New(repo, nil, nil, nil, WithProviderPolicy(policy))
	hostAgent := Provider("host-agent")
	if _, err := service.Update(context.Background(), "deadbeef", UpdateInput{Provider: &hostAgent}); err != ErrInvalidProvider {
		t.Fatalf("project update to host agent error = %v", err)
	}
}

func TestUpdateRejectsUnsupportedModeWhenProviderChanges(t *testing.T) {
	repo := &forkRepository{source: Meta{
		ID:       "deadbeef",
		Provider: ProviderClaude,
		Mode:     "plan",
	}}
	service := New(
		repo,
		nil,
		nil,
		nil,
		WithProviderPolicy(forkProviderPolicy{
			string(ProviderClaude): true,
			string(ProviderCodex):  true,
		}),
	)
	nextProvider := ProviderCodex
	staleMode := "plan"
	_, err := service.Update(context.Background(), repo.source.ID, UpdateInput{
		Provider: &nextProvider,
		Mode:     &staleMode,
	})
	if !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("update error = %v, want ErrInvalidMode", err)
	}
}

func TestCreateAndSameProviderUpdateRejectUnsupportedMode(t *testing.T) {
	repo := &forkRepository{}
	service := New(
		repo,
		nil,
		nil,
		nil,
		WithProviderPolicy(forkProviderPolicy{string(ProviderClaude): true}),
	)
	_, err := service.Create(context.Background(), CreateInput{
		Provider: ProviderClaude,
		Mode:     "plan",
	})
	if !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("create error = %v, want ErrInvalidMode", err)
	}

	repo.source = Meta{ID: "deadbeef", Provider: ProviderClaude, Mode: "default"}
	staleMode := "plan"
	_, err = service.Update(context.Background(), repo.source.ID, UpdateInput{
		Mode: &staleMode,
	})
	if !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("update error = %v, want ErrInvalidMode", err)
	}
}

func TestForkPreservesUnsupportedModeWithoutChangingSemantics(t *testing.T) {
	repo := &forkRepository{source: Meta{
		ID:       "deadbeef",
		Title:    "Source",
		Provider: ProviderClaude,
		Mode:     "plan",
	}}
	service := New(repo, nil, nil, nil)

	forked, err := service.Fork(context.Background(), repo.source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if forked.Mode != "plan" {
		t.Fatalf("forked mode = %q", forked.Mode)
	}
}

func TestUpdateRequiresIdleTransitionBarrier(t *testing.T) {
	repo := &forkRepository{source: Meta{
		ID:       "deadbeef",
		Title:    "Before",
		Provider: ProviderCodex,
		Mode:     string(agent.RunModeDefault),
	}}
	runs := &rejectsIdleChangeRunController{}
	service := New(repo, nil, nil, runs)
	title := "Must not win the race"
	_, err := service.Update(context.Background(), repo.source.ID, UpdateInput{Title: &title})
	if !errors.Is(err, ErrChatRunning) {
		t.Fatalf("update error = %v, want ErrChatRunning", err)
	}
	if repo.source.Title != "Before" {
		t.Fatalf("running chat was mutated: %#v", repo.source)
	}
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

func (r *forkRepository) Update(_ context.Context, _ ID, mutate func(*Meta)) (Meta, error) {
	meta := r.source
	if meta.ID == "" {
		meta = Meta{ID: "deadbeef", Provider: ProviderCodex}
	}
	mutate(&meta)
	r.source = meta
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
