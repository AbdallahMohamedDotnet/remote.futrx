package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/service/agent/module"
)

type Service struct {
	repo         Repository
	copiedEvents CopiedEventAppender
	projects     ProjectResolver
	tmux         TmuxResolver
	runs         RunController
	sessions     SessionPolicy
	providers    ProviderPolicy
}

// SessionPolicy supplies provider-native behavior from the agent module
// catalog without coupling chat orchestration to concrete adapters.
type SessionPolicy interface {
	SupportsNativeFork(provider string) bool
}

type ProviderPolicy interface {
	HasProvider(provider string) bool
	SupportsScope(provider string, scope agentmodule.ExecutionScope) bool
	SupportsRunMode(provider string, mode agent.RunMode) bool
}

type defaultProviderPolicy interface {
	DefaultProvider(scope agentmodule.ExecutionScope) agent.ProviderID
}

// Option configures an optional chat-service collaborator.
type Option func(*Service)

// WithCopiedEventAppender preserves copied history without raising the side
// effects reserved for newly produced events.
func WithCopiedEventAppender(appender CopiedEventAppender) Option {
	return func(service *Service) {
		service.copiedEvents = appender
	}
}

func WithSessionPolicy(policy SessionPolicy) Option {
	return func(service *Service) {
		service.sessions = policy
	}
}

func WithProviderPolicy(policy ProviderPolicy) Option {
	return func(service *Service) {
		service.providers = policy
	}
}

func New(
	repo Repository,
	projects ProjectResolver,
	tmux TmuxResolver,
	runs RunController,
	options ...Option,
) *Service {
	service := &Service{
		repo:     repo,
		projects: projects,
		tmux:     tmux,
		runs:     runs,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) List(ctx context.Context) ([]Meta, error) {
	metas, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range metas {
		metas[i] = s.withRunning(metas[i])
	}
	return metas, nil
}

func (s *Service) Get(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	meta, err := s.repo.Get(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	return s.withRunning(meta), nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Meta, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "New chat"
	}

	provider, ok := s.providerForScope(in.Provider, in.ProjectID)
	if !ok {
		return Meta{}, ErrInvalidProvider
	}
	mode, err := s.modeForProvider(provider, in.Mode)
	if err != nil {
		return Meta{}, err
	}

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

	meta, err := s.repo.Create(ctx, Meta{
		Title:           title,
		Provider:        provider,
		TmuxSession:     in.TmuxSession,
		Cwd:             cwd,
		Model:           in.Model,
		Mode:            string(mode),
		ReasoningEffort: NormalizeReasoningEffort(in.ReasoningEffort),
		ServiceTier:     NormalizeServiceTier(in.ServiceTier),
		ProjectID:       in.ProjectID,
		SelectedSkills:  NormalizeSelectedSkills(in.SelectedSkills, provider),
	})
	if err != nil {
		return Meta{}, err
	}
	return s.withRunning(meta), nil
}

// Fork creates an independent copy of a chat from its latest state: same
// metadata and full visible history, plus a pending fork of the underlying
// agent session. The fork materializes on the next prompt through each
// provider's native fork mechanism, so the parent is never mutated.
func (s *Service) Fork(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	src, err := s.repo.Get(ctx, id)
	if err != nil {
		return Meta{}, err
	}
	if !s.validProviderScope(src.Provider, src.ProjectID) {
		return Meta{}, ErrInvalidProvider
	}
	events, err := s.repo.ReadEvents(ctx, id)
	if err != nil {
		return Meta{}, err
	}

	title := strings.TrimSpace(src.Title)
	if title == "" {
		title = "Untitled"
	}

	// Only pend a fork if there is a session to fork from; otherwise the copy
	// just starts fresh on first prompt. TmuxSession is intentionally not
	// copied — a fork must not hijack the parent's terminal.
	sessions := src.SessionSnapshot()
	nativeFork := s.sessions != nil && s.sessions.SupportsNativeFork(string(src.Provider))
	forkPending := nativeFork && src.SessionID(src.Provider) != ""
	if !nativeFork {
		delete(sessions, src.Provider)
	}
	forkMeta := Meta{
		Title:           title + " (fork)",
		Provider:        src.Provider,
		Sessions:        sessions,
		Cwd:             src.Cwd,
		Model:           src.Model,
		Mode:            storedMode(src.Mode),
		ReasoningEffort: src.ReasoningEffort,
		ServiceTier:     src.ServiceTier,
		ProjectID:       src.ProjectID,
		SelectedSkills:  src.SelectedSkills,
		ForkPending:     forkPending,
	}
	forkMeta.NormalizeSessions()
	forked, err := s.repo.Create(ctx, forkMeta)
	if err != nil {
		return Meta{}, err
	}

	// Copy the visible history so the fork opens on the same conversation.
	// Zero seq so the store assigns fresh, monotonic sequence numbers.
	for _, ev := range events {
		ev.Seq = 0
		if _, err := s.appendCopiedEvent(ctx, forked.ID, ev); err != nil {
			return Meta{}, err
		}
	}

	return s.withRunning(forked), nil
}

func (s *Service) appendCopiedEvent(ctx context.Context, id ID, event Event) (Event, error) {
	if s.copiedEvents != nil {
		return s.copiedEvents.AppendCopiedEvent(ctx, id, event)
	}
	return s.repo.AppendEvent(ctx, id, event)
}

func (s *Service) Update(ctx context.Context, id ID, in UpdateInput) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}

	var meta Meta
	change := func() error {
		current, err := s.repo.Get(ctx, id)
		if err != nil {
			return err
		}
		nextProvider := current.Provider
		if in.Provider != nil {
			var valid bool
			nextProvider, valid = s.providerForScope(*in.Provider, current.ProjectID)
			if !valid {
				return ErrInvalidProvider
			}
		}
		providerChanged := nextProvider != current.Provider
		var nextMode *agent.RunMode
		if in.Mode != nil {
			mode, modeErr := s.modeForProvider(nextProvider, *in.Mode)
			if modeErr != nil {
				return modeErr
			}
			nextMode = &mode
		} else if providerChanged {
			mode := agent.RunModeDefault
			nextMode = &mode
		}

		invalidMode := false
		meta, err = s.repo.Update(ctx, id, func(m *Meta) {
			providerChangedNow := in.Provider != nil && nextProvider != m.Provider
			effectiveProvider := m.Provider
			if in.Provider != nil {
				effectiveProvider = nextProvider
			}
			if nextMode != nil && !s.supportsRunMode(effectiveProvider, *nextMode) {
				invalidMode = true
				return
			}
			if in.Title != nil {
				m.Title = strings.TrimSpace(*in.Title)
			}
			if in.Cwd != nil {
				m.Cwd = *in.Cwd
			}
			if in.Provider != nil {
				if providerChangedNow {
					m.SelectedSkills = nil
				}
				m.Provider = nextProvider
			}
			if in.Model != nil {
				m.Model = *in.Model
			}
			if nextMode != nil {
				m.Mode = string(*nextMode)
			}
			if in.ReasoningEffort != nil {
				m.ReasoningEffort = NormalizeReasoningEffort(*in.ReasoningEffort)
			}
			if in.ServiceTier != nil {
				m.ServiceTier = NormalizeServiceTier(*in.ServiceTier)
			}
			if in.SelectedSkills != nil {
				m.SelectedSkills = NormalizeSelectedSkills(*in.SelectedSkills, m.Provider)
			}
		})
		if err != nil {
			return err
		}
		if invalidMode {
			return ErrInvalidMode
		}
		return nil
	}
	if s.runs != nil {
		idle, err := s.runs.WhileIdle(id, change)
		if err != nil {
			return Meta{}, err
		}
		if !idle {
			return Meta{}, ErrChatRunning
		}
	} else if err := change(); err != nil {
		return Meta{}, err
	}
	return s.withRunning(meta), nil
}

func (s *Service) modeForProvider(provider Provider, value string) (agent.RunMode, error) {
	mode, err := ParseMode(value)
	if err != nil || !s.supportsRunMode(provider, mode) {
		return "", ErrInvalidMode
	}
	return mode, nil
}

func (s *Service) supportsRunMode(provider Provider, mode agent.RunMode) bool {
	if s.providers == nil {
		return mode == agent.RunModeDefault
	}
	return s.providers.SupportsRunMode(string(provider), mode)
}

func storedMode(value string) string {
	mode, err := ParseMode(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return string(mode)
}

func (s *Service) validProviderScope(provider Provider, projectID ProjectID) bool {
	if s.providers == nil {
		return true
	}
	if !s.providers.HasProvider(string(provider)) {
		return false
	}
	scope := agentmodule.ScopeHost
	if projectID != "" {
		scope = agentmodule.ScopeProject
	}
	return s.providers.SupportsScope(string(provider), scope)
}

func (s *Service) providerForScope(input Provider, projectID ProjectID) (Provider, bool) {
	scope := agentmodule.ScopeHost
	if projectID != "" {
		scope = agentmodule.ScopeProject
	}
	normalized := agent.NormalizeProviderID(string(input))
	if normalized == "" {
		normalized = ProviderCodex
		if defaults, ok := s.providers.(defaultProviderPolicy); ok {
			normalized = defaults.DefaultProvider(scope)
		}
	}
	if !agent.ValidProviderID(normalized) || !s.validProviderScope(normalized, projectID) {
		return "", false
	}
	return normalized, true
}

func (s *Service) MarkRead(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	meta, err := s.repo.Update(ctx, id, func(m *Meta) {
		m.LastReadAt = m.LastMessageAt
	})
	if err != nil {
		return Meta{}, err
	}
	return s.withRunning(meta), nil
}

func (s *Service) MarkUnread(ctx context.Context, id ID) (Meta, error) {
	if !ValidID(id) {
		return Meta{}, ErrInvalidID
	}
	meta, err := s.repo.Update(ctx, id, func(m *Meta) {
		if m.LastMessageAt > 0 {
			m.LastReadAt = m.LastMessageAt - 1
		} else {
			m.LastReadAt = 0
		}
	})
	if err != nil {
		return Meta{}, err
	}
	return s.withRunning(meta), nil
}

func (s *Service) withRunning(meta Meta) Meta {
	if s.runs != nil {
		meta.Running = s.runs.IsRunning(meta.ID)
	}
	return meta
}

func (s *Service) Delete(ctx context.Context, id ID) error {
	if !ValidID(id) {
		return ErrInvalidID
	}
	if s.runs != nil {
		return s.runs.CancelAndWhileIdle(ctx, id, func() error {
			return s.repo.Delete(ctx, id)
		})
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
	var events []Event
	change := func() error {
		current, err := s.repo.Get(ctx, id)
		if err != nil {
			return err
		}
		currentEvents, err := s.repo.ReadEvents(ctx, id)
		if err != nil {
			return err
		}
		receipts, changed, err := current.PromptReceipts.withAcceptedEvents(currentEvents, beforeT)
		if err != nil {
			return err
		}
		if changed {
			if _, err := s.repo.Update(ctx, id, func(meta *Meta) {
				meta.PromptReceipts = receipts
			}); err != nil {
				return err
			}
		}
		events, err = s.repo.TruncateEventsBefore(ctx, id, beforeT)
		return err
	}
	if s.runs != nil {
		idle, err := s.runs.WhileIdle(id, change)
		if err != nil {
			return nil, err
		}
		if !idle {
			return nil, ErrChatRunning
		}
	} else if err := change(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Service) UploadTarget(ctx context.Context, id ID) (string, error) {
	if !ValidID(id) {
		return "", ErrInvalidID
	}
	meta, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}

	// Anchor uploads at the project's stable workspace root, not the chat's
	// (possibly tmux-driven) live cwd: a fixed root keeps the stored path
	// constant so the frontend can predict it exactly, and the .uploads/
	// subdir isolates attachments from the source tree.
	root := meta.Cwd
	if meta.ProjectID != "" && s.projects != nil {
		if ws, err := s.projects.WorkspaceForProject(ctx, meta.ProjectID); err == nil && ws != "" {
			root = ws
		}
	}

	if root == "" {
		root = os.Getenv("HOME")
		if root == "" {
			root = "/root"
		}
	}
	return filepath.Join(root, ".uploads"), nil
}
