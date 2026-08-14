// Package agentcatalog aggregates the provider-specific capability catalogs
// exposed by the registered agent CLIs.
//
// Catalog discovery may start several comparatively expensive CLI probes (for
// example, Codex app-server and provider model-list commands) whenever a chat
// composer mounts or changes projects. The short-lived cache exists only to
// avoid repeating those processes for nearby requests. Entries are scoped to
// the host or to a project ID and container name, expire after 30 seconds, and
// can be bypassed with ListQuery.Refresh.
//
// The cache is not a source of truth: provider CLIs remain authoritative. The
// current implementation also caches fallback results and does not invalidate
// entries immediately after CLI upgrades or authentication changes. If this
// becomes observable, prefer coalescing concurrent discovery requests or
// caching only successful live results rather than increasing the TTL.
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

type ListQuery struct {
	ProjectID     serviceproject.ID
	SessionCookie string
	Refresh       bool
}

type Catalog struct {
	agents   *agent.Registry
	projects ProjectCatalog
	auth     Authorizer
	cache    *catalogCache
}

func New(agents *agent.Registry, projects ProjectCatalog, auth Authorizer) *Catalog {
	return &Catalog{
		agents: agents, projects: projects, auth: auth, cache: newCatalogCache(),
	}
}

func (c *Catalog) List(ctx context.Context, query ListQuery) ([]agent.Capabilities, error) {
	containerName := ""
	cacheKey := "host"
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
		cacheKey = "project:" + string(project.ID) + ":" + containerName
	}

	if !query.Refresh {
		if cached, ok := c.cache.load(cacheKey); ok {
			return cached, nil
		}
	}
	providers := c.agents.Providers()
	result := make([]agent.Capabilities, len(providers))
	var wait sync.WaitGroup
	for index, provider := range providers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			caps, err := provider.Capabilities(ctx, agent.CapabilityRequest{ContainerName: containerName})
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
	c.cache.store(cacheKey, result)
	return cloneCapabilities(result), nil
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
