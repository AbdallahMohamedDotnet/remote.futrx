// Package profiles owns the configured agent profiles shared by container
// capabilities.
package profiles

import (
	"sync"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// Registry provides a copy-on-write boundary around configured profiles.
// Callers can neither mutate stored profiles nor observe later configuration
// changes through an earlier snapshot.
type Registry struct {
	mu       sync.RWMutex
	profiles []provisioning.Profile
}

// NewRegistry returns an empty profile registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Replace atomically replaces the configured profiles with defensive copies.
func (r *Registry) Replace(profiles []provisioning.Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles = clone(profiles)
}

// Snapshot returns defensive copies of the configured profiles.
func (r *Registry) Snapshot() []provisioning.Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.profiles)
}

func clone(profiles []provisioning.Profile) []provisioning.Profile {
	out := make([]provisioning.Profile, len(profiles))
	for i := range profiles {
		out[i] = profiles[i].Clone()
	}
	return out
}
