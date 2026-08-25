package module

import (
	"errors"
	"fmt"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/agent/auth"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

var (
	ErrInvalidCatalog = errors.New("invalid agent module catalog")
	ErrNoAccessGate   = errors.New("agent module catalog has no access-gate provider")
)

// Catalog is the immutable composition source for all configured agents.
// Order is intentional and is preserved in provisioning, runtime, auth, and
// capability views.
type Catalog struct {
	factories []Factory
	byID      map[agent.ProviderID]int
}

func NewCatalog(factories ...Factory) (*Catalog, error) {
	catalog := &Catalog{
		factories: append([]Factory(nil), factories...),
		byID:      make(map[agent.ProviderID]int, len(factories)),
	}
	if len(catalog.factories) == 0 {
		return nil, fmt.Errorf("%w: no factories", ErrInvalidCatalog)
	}
	stateDevices := make(map[string]agent.ProviderID)
	stateHosts := make(map[string]agent.ProviderID)
	stateTargets := make(map[string]agent.ProviderID)
	var defaultProvider agent.ProviderID
	for index, factory := range catalog.factories {
		descriptor := factory.Descriptor()
		if err := validateDescriptor(descriptor); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCatalog, err)
		}
		if factory.build == nil {
			return nil, fmt.Errorf("%w: provider %q has no builder", ErrInvalidCatalog, descriptor.ID)
		}
		if _, exists := catalog.byID[descriptor.ID]; exists {
			return nil, fmt.Errorf("%w: provider %q is duplicated", ErrInvalidCatalog, descriptor.ID)
		}
		if descriptor.Default {
			if defaultProvider != "" {
				return nil, fmt.Errorf(
					"%w: providers %q and %q are both defaults",
					ErrInvalidCatalog,
					defaultProvider,
					descriptor.ID,
				)
			}
			defaultProvider = descriptor.ID
		}
		if descriptor.Profile != nil {
			for _, state := range descriptor.Profile.PersistentState {
				if owner, exists := stateDevices[state.Device]; exists {
					return nil, duplicateStateMountError(descriptor.ID, owner, "device", state.Device)
				}
				if owner, exists := stateHosts[state.HostDirectory]; exists {
					return nil, duplicateStateMountError(descriptor.ID, owner, "host directory", state.HostDirectory)
				}
				for target, owner := range stateTargets {
					if statePathsOverlap(target, state.ContainerPath) {
						return nil, duplicateStateMountError(descriptor.ID, owner, "container path", state.ContainerPath)
					}
				}
				stateDevices[state.Device] = descriptor.ID
				stateHosts[state.HostDirectory] = descriptor.ID
				stateTargets[state.ContainerPath] = descriptor.ID
			}
		}
		catalog.byID[descriptor.ID] = index
	}
	return catalog, nil
}

func statePathsOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func duplicateStateMountError(provider, owner agent.ProviderID, field, value string) error {
	return fmt.Errorf(
		"%w: providers %q and %q share persistent-state %s %q",
		ErrInvalidCatalog,
		owner,
		provider,
		field,
		value,
	)
}

func (c *Catalog) Descriptors() []Descriptor {
	if c == nil {
		return nil
	}
	descriptors := make([]Descriptor, len(c.factories))
	for index, factory := range c.factories {
		descriptors[index] = factory.Descriptor()
	}
	return descriptors
}

func (c *Catalog) Descriptor(provider string) (Descriptor, bool) {
	if c == nil {
		return Descriptor{}, false
	}
	index, ok := c.byID[agent.ProviderID(provider)]
	if !ok {
		return Descriptor{}, false
	}
	return c.factories[index].Descriptor(), true
}

func (c *Catalog) HasProvider(provider string) bool {
	if c == nil {
		return false
	}
	_, ok := c.byID[agent.ProviderID(provider)]
	return ok
}

// SupportsScope reports whether a configured provider may execute in the
// requested environment. Membership alone is insufficient: a host-only
// adapter must never be launched for a project chat, and vice versa.
func (c *Catalog) SupportsScope(provider string, scope ExecutionScope) bool {
	descriptor, ok := c.Descriptor(provider)
	if !ok {
		return false
	}
	for _, configured := range descriptor.ExecutionScopes {
		if configured == scope {
			return true
		}
	}
	return false
}

// DefaultProvider returns the explicitly preferred provider for a scope, or
// the first compatible module when no preference is declared. Catalog order
// is deterministic, so this fallback is stable.
func (c *Catalog) DefaultProvider(scope ExecutionScope) agent.ProviderID {
	if c == nil {
		return ""
	}
	var first agent.ProviderID
	for _, factory := range c.factories {
		descriptor := factory.Descriptor()
		if !c.SupportsScope(string(descriptor.ID), scope) {
			continue
		}
		if first == "" {
			first = descriptor.ID
		}
		if descriptor.Default {
			return descriptor.ID
		}
	}
	return first
}

// AccessReady reports whether any module declared as an onboarding gate can
// run. No-auth modules are immediately ready; managed flows require a live
// authenticated binding. External flows cannot be gate providers because the
// platform has no authoritative status signal for them.
func (c *Catalog) AccessReady(bindings *agentauth.Registry) bool {
	if c == nil {
		return false
	}
	for _, factory := range c.factories {
		descriptor := factory.Descriptor()
		if !descriptor.SatisfiesAccessGate {
			continue
		}
		if descriptor.Auth == AuthNone {
			return true
		}
		binding, ok := bindings.Lookup(descriptor.ID)
		if ok && binding.Authenticated() {
			return true
		}
	}
	return false
}

// ValidateAccessGate rejects a catalog that would leave an authenticated
// deployment permanently behind the provider-onboarding gate. Auth-disabled
// consumers do not need to call this validation.
func (c *Catalog) ValidateAccessGate() error {
	if c != nil {
		for _, factory := range c.factories {
			if factory.Descriptor().SatisfiesAccessGate {
				return nil
			}
		}
	}
	return ErrNoAccessGate
}

func (c *Catalog) LegacySkillRoots(provider string) []string {
	descriptor, ok := c.Descriptor(provider)
	if !ok {
		return nil
	}
	return append([]string(nil), descriptor.LegacySkillRoots...)
}

// SupportsNativeFork lets orchestration decide whether a copied chat may send
// the provider's session ID back with a native fork request.
func (c *Catalog) SupportsNativeFork(provider string) bool {
	descriptor, ok := c.Descriptor(provider)
	return ok && descriptor.Features.Sessions.Fork
}

func (c *Catalog) Profiles() []provisioning.Profile {
	if c == nil {
		return nil
	}
	profiles := make([]provisioning.Profile, 0, len(c.factories))
	for _, factory := range c.factories {
		descriptor := factory.Descriptor()
		if profile := descriptor.Profile; profile != nil && c.SupportsScope(string(descriptor.ID), ScopeProject) {
			profiles = append(profiles, profile.Clone())
		}
	}
	return profiles
}

// HostProfiles returns local CLI policies in module order. Host-only remote
// integrations may omit a profile and therefore require no host installation.
func (c *Catalog) HostProfiles() []provisioning.Profile {
	if c == nil {
		return nil
	}
	profiles := make([]provisioning.Profile, 0, len(c.factories))
	for _, factory := range c.factories {
		descriptor := factory.Descriptor()
		if profile := descriptor.Profile; profile != nil && c.SupportsScope(string(descriptor.ID), ScopeHost) {
			profiles = append(profiles, profile.Clone())
		}
	}
	return profiles
}

// Runtime contains the two registries built from the same validated factories.
// Keeping this operation atomic prevents provider/profile/auth drift.
type Runtime struct {
	Providers *agent.Registry
	Auth      *agentauth.Registry
}

func (c *Catalog) Build(deps Dependencies) (Runtime, error) {
	if c == nil {
		return Runtime{}, fmt.Errorf("%w: catalog is nil", ErrInvalidCatalog)
	}
	providers := agent.NewRegistry()
	auth := agentauth.NewRegistry()
	for _, factory := range c.factories {
		components, err := factory.buildComponents(deps)
		if err != nil {
			return Runtime{}, err
		}
		if err := providers.Register(components.Provider); err != nil {
			return Runtime{}, err
		}
		if components.Auth != nil {
			if err := auth.Register(*components.Auth); err != nil {
				return Runtime{}, err
			}
		}
	}
	return Runtime{Providers: providers, Auth: auth}, nil
}
