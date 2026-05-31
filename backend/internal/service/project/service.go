package project

import (
	"context"
	"log"
	"strings"
)

type Service struct {
	repo       Repository
	containers ContainerManager
	secrets    SecretsRepository
}

func New(repo Repository, containers ContainerManager, secrets SecretsRepository) *Service {
	return &Service{repo: repo, containers: containers, secrets: secrets}
}

func (s *Service) ListSecrets(ctx context.Context, id ID) ([]Secret, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	if s.secrets == nil {
		return nil, nil
	}
	return s.secrets.List(ctx, id)
}

func (s *Service) SetSecret(ctx context.Context, id ID, key, value string) (Secret, error) {
	if !ValidID(id) {
		return Secret{}, ErrInvalidID
	}
	if !ValidSecretKey(key) {
		return Secret{}, ErrInvalidSecretKey
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return Secret{}, err
	}
	if s.secrets == nil {
		return Secret{}, ErrSecretsUnavailable
	}
	return s.secrets.Set(ctx, id, key, value)
}

func (s *Service) DeleteSecret(ctx context.Context, id ID, key string) error {
	if !ValidID(id) {
		return ErrInvalidID
	}
	if !ValidSecretKey(key) {
		return ErrInvalidSecretKey
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	if s.secrets == nil {
		return ErrSecretsUnavailable
	}
	return s.secrets.Delete(ctx, id, key)
}

func (s *Service) List(ctx context.Context) ([]Meta, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (Meta, error) {
	if !ValidSlug(slug) {
		return Meta{}, ErrNotFound
	}
	return s.repo.GetBySlug(ctx, slug)
}

func (s *Service) WorkspaceForProject(ctx context.Context, id ID) (string, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return p.Cwd, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Meta, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Meta{}, ErrNameRequired
	}

	m, err := s.repo.Create(ctx, Meta{
		Name:   name,
		Slug:   Slugify(name),
		Status: StatusProvisioning,
	})
	if err != nil {
		return Meta{}, err
	}

	if s.containers != nil {
		if err := s.containers.Launch(ctx, m); err != nil {
			log.Printf("projects: launch %s failed: %v", m.ContainerName, err)
			return s.repo.SetStatus(ctx, m.ID, StatusError, err.Error())
		}
	}
	return s.repo.SetStatus(ctx, m.ID, StatusRunning, "")
}

func (s *Service) Update(ctx context.Context, id ID, in UpdateInput) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	return s.repo.Update(ctx, id, func(m *Meta) {
		if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
			m.Name = strings.TrimSpace(*in.Name)
		}
	})
}

func (s *Service) Delete(ctx context.Context, id ID) error {
	if !ValidID(id) {
		return ErrInvalidID
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if s.containers != nil && m.ContainerName != "" {
		if err := s.containers.Delete(ctx, m.ContainerName); err != nil {
			log.Printf("projects: delete container %s: %v", m.ContainerName, err)
		}
	}
	if s.secrets != nil {
		if err := s.secrets.DeleteAll(ctx, id); err != nil {
			log.Printf("projects: delete secrets %s: %v", id, err)
		}
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) Start(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	if s.containers != nil {
		if err := s.containers.Start(ctx, m.ContainerName); err != nil {
			return s.repo.SetStatus(ctx, id, StatusError, err.Error())
		}
	}
	return s.repo.SetStatus(ctx, id, StatusRunning, "")
}

func (s *Service) Stop(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	if s.containers != nil {
		if err := s.containers.Stop(ctx, m.ContainerName); err != nil {
			return s.repo.SetStatus(ctx, id, StatusError, err.Error())
		}
	}
	return s.repo.SetStatus(ctx, id, StatusStopped, "")
}

func (s *Service) InspectContainer(ctx context.Context, id ID) (ContainerInspect, error) {
	if !ValidID(id) {
		return ContainerInspect{}, ErrInvalidID
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return ContainerInspect{}, err
	}
	if s.containers == nil || m.ContainerName == "" {
		return ContainerInspect{Name: m.ContainerName}, nil
	}
	return s.containers.Inspect(ctx, m.ContainerName)
}

func (s *Service) Reconcile(ctx context.Context) error {
	if s.containers == nil || !s.containers.Available() {
		return nil
	}
	metas, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, m := range metas {
		state, err := s.containers.State(ctx, m.ContainerName)
		if err != nil {
			continue
		}
		want := statusForContainerState(state)
		if want != m.Status {
			if _, err := s.repo.SetStatus(ctx, m.ID, want, ""); err != nil {
				log.Printf("projects: reconcile %s: %v", m.ID, err)
			}
		}
	}
	return nil
}

func statusForContainerState(state ContainerState) Status {
	switch state {
	case ContainerStateRunning:
		return StatusRunning
	case ContainerStateStopped:
		return StatusStopped
	case ContainerStateMissing:
		return StatusMissing
	default:
		return StatusUnknown
	}
}
