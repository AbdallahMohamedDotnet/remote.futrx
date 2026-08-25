package module

import (
	"errors"
	"fmt"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

var ErrInvalidCatalog = errors.New("invalid agent module catalog")

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
		catalog.byID[descriptor.ID] = index
	}
	return catalog, nil
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

func (c *Catalog) Profiles() []provisioning.Profile {
	if c == nil {
		return nil
	}
	profiles := make([]provisioning.Profile, 0, len(c.factories))
	for _, factory := range c.factories {
		if profile := factory.Descriptor().Profile; profile != nil {
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
