package agentcatalog

import (
	"context"
	"sync"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

type catalogTestProvider struct {
	id       agent.ProviderID
	label    string
	mu       sync.Mutex
	calls    int
	requests []agent.CapabilityRequest
}

func (p *catalogTestProvider) ID() agent.ProviderID { return p.id }
func (p *catalogTestProvider) Capabilities(_ context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.requests = append(p.requests, req)
	return agent.Capabilities{
		Provider: p.id, Label: p.label, Source: agent.CapabilitySourceLive,
		Models: []agent.ModelCapability{}, Modes: []agent.CapabilityOption{},
	}, nil
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

func TestListUsesRegistryOrderProjectContainerAndCache(t *testing.T) {
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
		t.Fatalf("refresh calls = claude:%d codex:%d", claude.calls, codex.calls)
	}
}
