package applications

import (
	"context"
	"github.com/futrx-com/remote.futrx.com/internal/rbac"
	"github.com/futrx-com/remote.futrx.com/internal/service/workspaceaccess"
	"github.com/futrx-com/remote.futrx.com/internal/service/workspaceide"
)

func WithWorkspaceAuthorizer(a workspaceaccess.Authorizer) Option {
	return func(s *Service) { s.workspaceAuthorizer = a }
}

// RequireWebAccess guards current application launch, direct HTTP and WebSocket
// admission. Instance identity comes from the store, never a forwarded header.
func (s *Service) RequireWebAccess(ctx context.Context, instanceID string) error {
	instance, ok, err := s.store.Get(ctx, instanceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	switch instance.ApplicationID {
	case "code-server":
		if instance.Scope != ScopeProject {
			return rbac.ErrDenied
		}
		return workspaceide.RequireAccess(ctx, s.workspaceAuthorizer, instance.ProjectID)
	case "terminal":
		if instance.Scope != ScopeProject {
			return rbac.ErrDenied
		}
		return workspaceaccess.Require(ctx, s.workspaceAuthorizer, "terminal", instance.ProjectID)
	}
	return nil
}
