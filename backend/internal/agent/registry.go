package agent

// Registry owns the configured agent implementations, keyed by their stable
// provider identifier. Agents are registered once at the composition root and
// looked up by application services at run time.
type Registry struct {
	providers map[ProviderID]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[ProviderID]Provider)}
}

func (r *Registry) Register(provider Provider) {
	if r.providers == nil {
		r.providers = make(map[ProviderID]Provider)
	}
	r.providers[provider.ID()] = provider
}

func (r *Registry) Lookup(id ProviderID) Provider {
	if r == nil {
		return nil
	}
	return r.providers[id]
}
