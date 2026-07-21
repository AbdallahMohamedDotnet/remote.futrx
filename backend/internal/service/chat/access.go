package chat

import (
	"context"
	"errors"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

var (
	ErrProjectMembershipRequired = errors.New("not a member of this project")
	ErrProjectAccessDenied       = errors.New("not a member of this chat's project")
)

type ProjectAccess interface {
	HasAccess(ctx context.Context, id serviceproject.ID, email string) (bool, error)
}

type AccessService struct {
	chats    *Service
	projects ProjectAccess
}

func NewAccessService(chats *Service, projects ProjectAccess) *AccessService {
	return &AccessService{chats: chats, projects: projects}
}

func (s *AccessService) List(ctx context.Context, email string, isAdmin bool) ([]Meta, error) {
	metas, err := s.chats.List(ctx)
	if err != nil {
		return nil, err
	}
	if isAdmin || s.projects == nil {
		return metas, nil
	}
	visible := make([]Meta, 0, len(metas))
	for _, meta := range metas {
		if meta.ProjectID == "" {
			visible = append(visible, meta)
			continue
		}
		hasAccess, err := s.projects.HasAccess(ctx, serviceproject.ID(meta.ProjectID), email)
		if err == nil && hasAccess {
			visible = append(visible, meta)
		}
	}
	return visible, nil
}

func (s *AccessService) Create(ctx context.Context, input CreateInput, email string, isAdmin bool) (Meta, error) {
	if input.ProjectID != "" {
		hasAccess, err := s.canAccessProject(ctx, serviceproject.ID(input.ProjectID), email, isAdmin)
		if err != nil {
			return Meta{}, err
		}
		if !hasAccess {
			return Meta{}, ErrProjectMembershipRequired
		}
	}
	return s.chats.Create(ctx, input)
}

func (s *AccessService) Get(ctx context.Context, id ID, email string, isAdmin bool) (Meta, error) {
	meta, err := s.chats.Get(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	if meta.ProjectID == "" {
		return meta, nil
	}
	hasAccess, err := s.canAccessProject(ctx, serviceproject.ID(meta.ProjectID), email, isAdmin)
	if err != nil {
		return Meta{}, err
	}
	if !hasAccess {
		return Meta{}, ErrProjectAccessDenied
	}
	return meta, nil
}

func (s *AccessService) canAccessProject(
	ctx context.Context,
	id serviceproject.ID,
	email string,
	isAdmin bool,
) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if s.projects == nil || email == "" {
		return false, nil
	}
	return s.projects.HasAccess(ctx, id, email)
}
