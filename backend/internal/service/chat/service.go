package chat

import (
	"context"
	"os"
	"strings"
)

type Service struct {
	repo     Repository
	projects ProjectResolver
	tmux     TmuxResolver
	runs     RunController
}

func New(repo Repository, projects ProjectResolver, tmux TmuxResolver, runs RunController) *Service {
	return &Service{
		repo:     repo,
		projects: projects,
		tmux:     tmux,
		runs:     runs,
	}
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

func (s *Service) Create(ctx context.Context, in CreateInput) (Meta, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "New chat"
	}

	mode := in.Mode
	if mode == "" {
		mode = "code"
	}
	provider := NormalizeProvider(in.Provider)

	cwd := strings.TrimSpace(in.Cwd)
	if cwd == "" && in.ProjectID != "" && s.projects != nil {
		projectCwd, err := s.projects.WorkspaceForProject(ctx, in.ProjectID)
		if err != nil {
			return Meta{}, err
		}
		cwd = projectCwd
	}

	if cwd == "" && in.TmuxSession != "" && s.tmux != nil {
		if !s.tmux.ValidName(in.TmuxSession) {
			return Meta{}, ErrInvalidTmuxSession
		}
		if tmuxCwd, err := s.tmux.Cwd(ctx, in.TmuxSession); err == nil {
			cwd = tmuxCwd
		}
	}

	return s.repo.Create(ctx, Meta{
		Title:           title,
		Provider:        provider,
		TmuxSession:     in.TmuxSession,
		Cwd:             cwd,
		Model:           in.Model,
		Mode:            mode,
		ReasoningEffort: NormalizeReasoningEffort(in.ReasoningEffort),
		ProjectID:       in.ProjectID,
	})
}

func (s *Service) Update(ctx context.Context, id ID, in UpdateInput) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}

	return s.repo.Update(ctx, id, func(m *Meta) {
		if in.Title != nil {
			m.Title = strings.TrimSpace(*in.Title)
		}
		if in.Cwd != nil {
			m.Cwd = *in.Cwd
		}
		if in.Provider != nil {
			m.Provider = NormalizeProvider(*in.Provider)
		}
		if in.Model != nil {
			m.Model = *in.Model
		}
		if in.Mode != nil {
			m.Mode = *in.Mode
		}
		if in.ReasoningEffort != nil {
			m.ReasoningEffort = NormalizeReasoningEffort(*in.ReasoningEffort)
		}
	})
}

func (s *Service) Delete(ctx context.Context, id ID) error {
	if !ValidID(id) {
		return ErrInvalidID
	}

	if s.runs != nil && s.runs.IsRunning(id) {
		if err := s.runs.Cancel(ctx, id); err != nil {
			return err
		}
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) Events(ctx context.Context, id ID) ([]Event, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	return s.repo.ReadEvents(ctx, id)
}

func (s *Service) EventPage(ctx context.Context, id ID, query EventPageQuery) (EventPage, error) {
	if !ValidID(id) {
		return EventPage{}, ErrInvalidID
	}
	return s.repo.ReadEventsPage(ctx, id, query)
}

func (s *Service) Rewind(ctx context.Context, id ID, beforeT int64) ([]Event, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	if beforeT <= 0 {
		return nil, ErrInvalidRewindTimestamp
	}
	if s.runs != nil && s.runs.IsRunning(id) {
		return nil, ErrChatRunning
	}
	return s.repo.TruncateEventsBefore(ctx, id, beforeT)
}

func (s *Service) UploadTarget(ctx context.Context, id ID) (string, error) {
	if !ValidID(id) {
		return "", ErrInvalidID
	}
	meta, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}

	cwd := meta.Cwd
	if meta.TmuxSession != "" && s.tmux != nil {
		if tmuxCwd, err := s.tmux.Cwd(ctx, meta.TmuxSession); err == nil && tmuxCwd != "" {
			cwd = tmuxCwd
		}
	}

	if cwd == "" {
		cwd = os.Getenv("HOME")
		if cwd == "" {
			cwd = "/root"
		}
	}
	return cwd, nil
}
