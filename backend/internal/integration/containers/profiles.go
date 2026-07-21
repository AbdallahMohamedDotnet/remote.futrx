package containers

import (
	"sync"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// profileRegistry owns the configured agent profiles and their copy-on-write
// boundary. Callers can neither mutate the stored profiles nor observe later
// configuration changes through an earlier snapshot.
type profileRegistry struct {
	mu       sync.RWMutex
	profiles []provisioning.Profile
}

func (r *profileRegistry) replace(profiles []provisioning.Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles = cloneProfiles(profiles)
}

func (r *profileRegistry) snapshot() []provisioning.Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneProfiles(r.profiles)
}

func cloneProfiles(profiles []provisioning.Profile) []provisioning.Profile {
	out := make([]provisioning.Profile, len(profiles))
	for i := range profiles {
		out[i] = profiles[i].Clone()
	}
	return out
}
