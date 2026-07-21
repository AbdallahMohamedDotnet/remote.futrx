package agent

import (
	"errors"
	"fmt"
)

var ErrInvalidProvider = errors.New("invalid agent provider")

// Registry owns the configured agent implementations, keyed by their stable
// provider identifier. Agents are registered once at the composition root and
// looked up by application services at run time.
type Registry struct {
	providers map[ProviderID]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[ProviderID]Provider)}
}

func (r *Registry) Register(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("%w: provider is nil", ErrInvalidProvider)
	}
	id := provider.ID()
	if id == "" {
		return fmt.Errorf("%w: provider ID is empty", ErrInvalidProvider)
	}
	if r.providers == nil {
		r.providers = make(map[ProviderID]Provider)
	}
	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("%w: provider %q is already registered", ErrInvalidProvider, id)
	}
	r.providers[id] = provider
	return nil
}

func (r *Registry) Lookup(id ProviderID) Provider {
	if r == nil {
		return nil
	}
	return r.providers[id]
}
