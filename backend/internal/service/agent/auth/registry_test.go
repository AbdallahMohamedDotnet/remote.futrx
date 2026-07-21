package auth

import (
	"errors"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
)

func TestRegistryPreservesOrderAndReturnsDefensiveCopies(t *testing.T) {
	registry := NewRegistry()
	claude := NewCodeBinding(agent.ProviderClaude, nil)
	codex := NewDeviceBinding[bindingTestStatus](agent.ProviderCodex, nil)
	for _, binding := range []Binding{claude, codex} {
		if err := registry.Register(binding); err != nil {
			t.Fatalf("Register(%q): %v", binding.ID(), err)
		}
	}

	bindings := registry.Bindings()
	if len(bindings) != 2 || bindings[0].ID() != agent.ProviderClaude || bindings[1].ID() != agent.ProviderCodex {
		t.Fatalf("Bindings = %#v", bindings)
	}
	bindings[0] = Binding{}
	if got := registry.Bindings()[0].ID(); got != agent.ProviderClaude {
		t.Fatalf("registry mutated through returned slice: first ID = %q", got)
	}
	if got, ok := registry.Lookup(agent.ProviderCodex); !ok || got.ID() != agent.ProviderCodex {
		t.Fatalf("Lookup(codex) = (%q, %v)", got.ID(), ok)
	}
}

func TestRegistryRejectsInvalidAndDuplicateBindings(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Binding{}); !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("Register(empty) error = %v", err)
	}
	binding := NewCodeBinding(agent.ProviderClaude, nil)
	if err := registry.Register(binding); err != nil {
		t.Fatalf("Register(claude): %v", err)
	}
	if err := registry.Register(binding); !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("Register(duplicate) error = %v", err)
	}
}
