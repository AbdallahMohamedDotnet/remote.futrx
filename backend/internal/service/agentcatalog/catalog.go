// Package agentcatalog aggregates the provider-specific capability catalogs
// exposed by the registered agent CLIs.
//
// Catalog discovery may start several comparatively expensive CLI probes (for
// example, Codex app-server and provider model-list commands). Simultaneous
// requests for the same host or project container share one in-flight probe.
// Completed live catalogs are cached here for 24 hours so every browser and
// device sees the same result without repeating provider discovery. A manual
// refresh bypasses and replaces the shared entry.
package agentcatalog

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

var (
	ErrProjectLookupUnavailable = errors.New("project lookup unavailable")
	ErrProjectNotFound          = errors.New("project not found")
	ErrAuthenticationRequired   = errors.New("authentication required")
	ErrProjectAccessDenied      = errors.New("project access denied")
)

type ProjectCatalog interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	HasAccess(ctx context.Context, id serviceproject.ID, email string) (bool, error)
}

type Authorizer interface {
	CurrentSession(cookieValue string) (*serviceauth.Session, error)
	IsAdmin(ctx context.Context, email string) (bool, error)
}

type CapabilityRegistry interface {
	CapabilityProviders() []agent.CapabilityProvider
}

type ListQuery struct {
	ProjectID     serviceproject.ID
	SessionCookie string
	Refresh       bool
}

type Catalog struct {
	agents   CapabilityRegistry
	projects ProjectCatalog
	auth     Authorizer
	cache    *catalogCache
	flights  *catalogFlights
}

func New(agents CapabilityRegistry, projects ProjectCatalog, auth Authorizer) *Catalog {
	return &Catalog{
		agents:   agents,
		projects: projects,
		auth:     auth,
		cache:    newCatalogCache(),
		flights:  newCatalogFlights(),
	}
}

func (c *Catalog) List(ctx context.Context, query ListQuery) ([]agent.Capabilities, error) {
	containerName := ""
	flightKey := "host"
	if query.ProjectID != "" {
		if c.projects == nil {
			return nil, ErrProjectLookupUnavailable
		}
		project, err := c.projects.Get(ctx, query.ProjectID)
		if err != nil {
			if errors.Is(err, serviceproject.ErrNotFound) {
				return nil, ErrProjectNotFound
			}
			return nil, err
		}
		if err := c.authorize(ctx, project.ID, query.SessionCookie); err != nil {
			return nil, err
		}
		containerName = project.ContainerName
		flightKey = "project:" + string(project.ID) + ":" + containerName
	}

	if !query.Refresh {
		if cached, ok := c.cache.load(flightKey); ok {
			return cached, nil
		}
	}

	return c.flights.do(ctx, flightKey, func(discoveryCtx context.Context) ([]agent.Capabilities, error) {
		// A catalog may have completed between the optimistic cache check and
		// this caller becoming the flight leader.
		if !query.Refresh {
			if cached, ok := c.cache.load(flightKey); ok {
				return cached, nil
			}
		}
		providers := c.agents.CapabilityProviders()
		result := make([]agent.Capabilities, len(providers))
		var wait sync.WaitGroup
		for index, provider := range providers {
			wait.Add(1)
			go func() {
				defer wait.Done()
				caps, err := provider.Capabilities(
					discoveryCtx,
					agent.CapabilityRequest{ContainerName: containerName},
				)
				if caps.Provider == "" {
					caps.Provider = provider.ID()
				}
				if caps.Source == "" {
					caps.Source = agent.CapabilitySourceFallback
				}
				if err != nil && caps.Warning == "" {
					caps.Warning = "Provider capabilities are temporarily unavailable"
				}
				if caps.Models == nil {
					caps.Models = []agent.ModelCapability{}
				}
				if caps.Modes == nil {
					caps.Modes = []agent.CapabilityOption{}
				}
				result[index] = caps
			}()
		}
		wait.Wait()
		c.cache.store(flightKey, result)
		return result, nil
	})
}

func (c *Catalog) authorize(ctx context.Context, projectID serviceproject.ID, cookie string) error {
	if c.auth == nil {
		return nil
	}
	session, err := c.auth.CurrentSession(cookie)
	if err != nil || session == nil {
		return ErrAuthenticationRequired
	}
	email := strings.ToLower(strings.TrimSpace(session.Email))
	if email == "" {
		return ErrAuthenticationRequired
	}
	isAdmin, _ := c.auth.IsAdmin(ctx, email)
	if isAdmin {
		return nil
	}
	hasAccess, err := c.projects.HasAccess(ctx, projectID, email)
	if err != nil {
		return err
	}
	if !hasAccess {
		return ErrProjectAccessDenied
	}
	return nil
}
