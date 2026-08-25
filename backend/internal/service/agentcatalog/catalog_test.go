package agentcatalog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

type catalogTestProvider struct {
	id       agent.ProviderID
	label    string
	mu       sync.Mutex
	calls    int
	requests []agent.CapabilityRequest
	entered  chan<- struct{}
	release  <-chan struct{}
}

func (p *catalogTestProvider) ID() agent.ProviderID { return p.id }
func (p *catalogTestProvider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	p.mu.Lock()
	p.calls++
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if p.entered != nil {
		p.entered <- struct{}{}
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return agent.Capabilities{Provider: p.id, Label: p.label}, ctx.Err()
		}
	}
	return agent.Capabilities{
		Provider: p.id, Label: p.label, Source: agent.CapabilitySourceLive,
		Models: []agent.ModelCapability{}, Modes: []agent.CapabilityOption{},
	}, nil
}

func TestListAppliesOneConfiguredTimeoutPerProvider(t *testing.T) {
	provider := &catalogTestProvider{
		id:      "slow-agent",
		label:   "Slow Agent",
		release: make(chan struct{}),
	}
	catalog := New(
		catalogTestRegistry{providers: []agent.CapabilityProvider{provider}},
		nil,
		nil,
		WithCapabilityTimeout(20*time.Millisecond),
	)

	started := time.Now()
	items, err := catalog.List(context.Background(), ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("capability discovery took %s despite configured timeout", elapsed)
	}
	if len(items) != 1 || items[0].Source != agent.CapabilitySourceFallback || items[0].Warning == "" {
		t.Fatalf("timed-out capabilities = %#v", items)
	}
}

type catalogTestRegistry struct {
	providers []agent.CapabilityProvider
}

func (r catalogTestRegistry) CapabilityProviders() []agent.CapabilityProvider {
	return r.providers
}

type catalogTestProjects struct {
	project serviceproject.Meta
}

func (p catalogTestProjects) Get(context.Context, serviceproject.ID) (serviceproject.Meta, error) {
	return p.project, nil
}

func (catalogTestProjects) HasAccess(context.Context, serviceproject.ID, string) (bool, error) {
	return true, nil
}

func TestListUsesRegistryOrderProjectContainerAndSharedCache(t *testing.T) {
	claude := &catalogTestProvider{id: agent.ProviderClaude, label: "Claude"}
	codex := &catalogTestProvider{id: agent.ProviderCodex, label: "Codex"}
	registry := catalogTestRegistry{providers: []agent.CapabilityProvider{claude, codex}}
	catalog := New(registry, catalogTestProjects{project: serviceproject.Meta{
		ID: "abcd", ContainerName: "remote-abcd", Status: serviceproject.StatusRunning,
	}}, nil)

	for call := 0; call < 2; call++ {
		items, err := catalog.List(context.Background(), ListQuery{ProjectID: "abcd"})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].Provider != agent.ProviderClaude || items[1].Provider != agent.ProviderCodex {
			t.Fatalf("items = %+v", items)
		}
	}
	if claude.calls != 1 || codex.calls != 1 {
		t.Fatalf("capability calls = claude:%d codex:%d", claude.calls, codex.calls)
	}
	if got := claude.requests[0].ContainerName; got != "remote-abcd" {
		t.Fatalf("container name = %q", got)
	}

	if _, err := catalog.List(context.Background(), ListQuery{ProjectID: "abcd", Refresh: true}); err != nil {
		t.Fatal(err)
	}
	if claude.calls != 2 || codex.calls != 2 {
		t.Fatalf("refreshed capability calls = claude:%d codex:%d", claude.calls, codex.calls)
	}
}

func TestListCoalescesOnlyOverlappingRequests(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	provider := &catalogTestProvider{
		id: agent.ProviderClaude, label: "Claude", entered: entered, release: release,
	}
	catalog := New(catalogTestRegistry{providers: []agent.CapabilityProvider{provider}}, nil, nil)
	firstDone := make(chan error, 1)
	go func() {
		_, err := catalog.List(context.Background(), ListQuery{})
		firstDone <- err
	}()
	<-entered

	waiterCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := catalog.List(waiterCtx, ListQuery{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("overlapping waiter error = %v", err)
	}
	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("overlapping capability calls = %d", calls)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.List(context.Background(), ListQuery{}); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	calls = provider.calls
	provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("cached capability calls = %d", calls)
	}
	if _, err := catalog.List(context.Background(), ListQuery{Refresh: true}); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	calls = provider.calls
	provider.mu.Unlock()
	if calls != 2 {
		t.Fatalf("refreshed capability calls = %d", calls)
	}
}

func TestCatalogTTLUsesShortRetryForDegradedResults(t *testing.T) {
	if got := catalogTTL([]agent.Capabilities{{Source: agent.CapabilitySourceLive}}); got != liveCatalogTTL {
		t.Fatalf("live TTL = %s", got)
	}
	if got := catalogTTL([]agent.Capabilities{{Source: agent.CapabilitySourceFallback}}); got != degradedCatalogTTL {
		t.Fatalf("fallback TTL = %s", got)
	}
	if got := catalogTTL([]agent.Capabilities{{Source: agent.CapabilitySourceLive, Warning: "partial"}}); got != degradedCatalogTTL {
		t.Fatalf("warning TTL = %s", got)
	}
}
