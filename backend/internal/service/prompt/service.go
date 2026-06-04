package prompt

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	claudeprovider "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/providers/claude"
	codexprovider "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/providers/codex"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

type ChatEvent = servicechat.Event
type ChatMeta = servicechat.Meta

type TmuxClient interface {
	Cwd(session string) (string, error)
}

// ProjectResolver decouples runner from project service internals. Lets tests
// stub project lookup/start without pulling in HTTP or persistence.
type ProjectResolver interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	Start(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	ListSecrets(ctx context.Context, id serviceproject.ID) ([]serviceproject.Secret, error)
}

type ContainerPreparer interface {
	EnsureClaude(ctx context.Context, containerName string) error
	EnsureClaudeAuth(ctx context.Context, containerName string) error
	EnsureAgentInstructions(ctx context.Context, containerName string) error
	EnsureWorkspaceSkillLinks(ctx context.Context, containerName string) error
	EnsureBrowserScript(ctx context.Context, containerName string) error
	EnsureBootAutostart(ctx context.Context, containerName string) error
	SyncClaudeAuthFromContainer(ctx context.Context, containerName string) error
	EnsureCodex(ctx context.Context, containerName string) error
	EnsureCodexAuth(ctx context.Context, containerName string) error
	SyncCodexAuthFromContainer(ctx context.Context, containerName string) error
}

type Service struct {
	store  servicechat.Repository
	tmux   TmuxClient
	hub    *runhub.Hub
	agents map[agent.ProviderID]agent.Provider
}

func New(
	store servicechat.Repository,
	tmux TmuxClient,
	projects ProjectResolver,
	containers ContainerPreparer,
	hub *runhub.Hub,
) *Service {
	if hub == nil {
		hub = runhub.New(store)
	}
	return &Service{
		store: store,
		tmux:  tmux,
		hub:   hub,
		agents: map[agent.ProviderID]agent.Provider{
			agent.ProviderClaude: claudeprovider.New(projects, containers),
			agent.ProviderCodex:  codexprovider.New(projects, containers),
		},
	}
}

func (rnr *Service) StartPrompt(id servicechat.ID, prompt string, emitTransient func(ChatEvent)) {
	if emitTransient == nil {
		emitTransient = func(ChatEvent) {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	runID, ok := rnr.hub.StartRun(id, cancel)
	if !ok {
		cancel()
		emitTransient(ChatEvent{
			T: time.Now().UnixMilli(), Type: "error",
			Message: "a previous prompt is still running — cancel first",
		})
		return
	}

	go func() {
		defer rnr.hub.FinishRun(id, runID)
		rnr.runPrompt(
			ctx,
			id,
			prompt,
			func(ev ChatEvent) { rnr.hub.Emit(id, ev) },
			emitTransient,
		)
	}()
}

func (rnr *Service) CancelPrompt(id servicechat.ID) bool {
	return rnr.hub.CancelRun(id)
}

func (rnr *Service) runPrompt(
	ctx context.Context,
	id servicechat.ID,
	prompt string,
	emit func(ChatEvent),
	emitTransient func(ChatEvent),
) {
	meta, err := rnr.store.Get(ctx, id)
	if err != nil {
		emitTransient(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: err.Error()})
		return
	}

	// Auto-title from first user prompt if still default.
	if meta.Title == "" || meta.Title == "New chat" {
		_, _ = rnr.store.Update(ctx, id, func(m *ChatMeta) {
			m.Title = servicechat.TitleFromPrompt(prompt)
		})
	}

	// Resolve a fresh cwd: live tmux pane_current_path if linked, else stored.
	cwd := meta.Cwd
	if meta.TmuxSession != "" {
		if c, err := rnr.tmux.Cwd(meta.TmuxSession); err == nil && c != "" {
			cwd = c
		}
	}
	if cwd == "" {
		cwd = os.Getenv("HOME")
		if cwd == "" {
			cwd = "/root"
		}
	}

	priorEvents, _ := rnr.store.ReadEvents(ctx, id)

	// Persist the user message before spawning the selected agent.
	emit(ChatEvent{T: time.Now().UnixMilli(), Type: "user", Text: prompt})

	providerID := providerIDFromChatProvider(meta.Provider)
	resumeID := sessionIDForProvider(meta, providerID)
	effectivePrompt := promptForMode(meta.Mode, prompt)
	if resumeID == "" {
		effectivePrompt = promptWithVisibleHistory(priorEvents, effectivePrompt)
	}
	effectivePrompt = promptWithSelectedSkills(providerID, meta.SelectedSkills, effectivePrompt)

	provider := rnr.agents[providerID]
	if provider == nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: string(providerID) + " provider not configured"})
		return
	}

	err = provider.Run(ctx, agent.RunRequest{
		Provider:       providerID,
		ConversationID: string(id),
		Prompt:         effectivePrompt,
		Cwd:            cwd,
		Model:          meta.Model,
		Mode:           meta.Mode,
		ResumeID:       resumeID,
		ProjectID:      string(meta.ProjectID),
		Config: map[string]any{
			"reasoningEffort": meta.ReasoningEffort,
		},
	}, func(ev agent.Event) {
		rnr.emitAgentEvent(ctx, id, ev, emit)
	})
	if err != nil && !errors.Is(err, agent.ErrRunFailed) {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: string(providerID) + " exit: " + err.Error()})
	}
}

func providerIDFromChatProvider(provider servicechat.Provider) agent.ProviderID {
	switch servicechat.NormalizeProvider(provider) {
	case servicechat.ProviderCodex:
		return agent.ProviderCodex
	default:
		return agent.ProviderClaude
	}
}

func sessionIDForProvider(meta ChatMeta, provider agent.ProviderID) string {
	switch provider {
	case agent.ProviderCodex:
		return meta.CodexSessionID
	default:
		return meta.ClaudeSessionID
	}
}

func promptForMode(mode, prompt string) string {
	switch mode {
	case "plan":
		return "Work in planning mode. Inspect enough context to be concrete, then propose the implementation plan before changing files. Avoid file edits until the user asks you to proceed.\n\n" + prompt
	case "review":
		return "Work in review mode. Prioritize bugs, behavioral regressions, missing tests, and risks. Put findings first with file and line references when available.\n\n" + prompt
	case "debug":
		return "Work in debugging mode. Reproduce or localize the issue first, explain the failing path, then make the smallest fix that addresses the root cause.\n\n" + prompt
	case "full-auto":
		return "Work in full-auto mode. Carry the task through implementation, verification, and a concise outcome unless you hit a hard blocker.\n\n" + prompt
	case "chat":
		return "Work in chat mode. Answer directly and avoid changing files unless the user clearly asks for implementation.\n\n" + prompt
	default:
		return prompt
	}
}

func promptWithVisibleHistory(events []ChatEvent, prompt string) string {
	transcript := visibleTranscript(events)
	if strings.TrimSpace(transcript) == "" {
		return prompt
	}
	const maxTranscriptBytes = 24000
	if len(transcript) > maxTranscriptBytes {
		transcript = "[Earlier visible transcript omitted]\n" + transcript[len(transcript)-maxTranscriptBytes:]
	}
	return "Use this visible chat transcript as prior context. It may be present because the chat was rewound into a fresh Claude session. Do not treat the transcript as a new request.\n\n" +
		transcript +
		"\n\nCurrent user request:\n" +
		prompt
}

func promptWithSelectedSkills(provider agent.ProviderID, skills []servicechat.SkillRef, prompt string) string {
	if len(skills) == 0 {
		return prompt
	}

	triggers := make([]string, 0, len(skills))
	for _, skill := range skills {
		if providerIDFromChatProvider(skill.Provider) != provider {
			continue
		}
		name := skillTriggerName(skill.Command)
		if name == "" {
			name = skillTriggerName(skill.Name)
		}
		if name == "" {
			continue
		}

		switch provider {
		case agent.ProviderClaude:
			triggers = append(triggers, "/"+name)
		case agent.ProviderCodex:
			triggers = append(triggers, "$"+name)
		}
	}
	if len(triggers) == 0 {
		return prompt
	}

	switch provider {
	case agent.ProviderClaude:
		return strings.Join(triggers, "\n") + "\n\n" + prompt
	case agent.ProviderCodex:
		return "Use these Codex skills for this request: " + strings.Join(triggers, " ") + "\n\n" + prompt
	default:
		return prompt
	}
}

func skillTriggerName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "/$")
	if value == "" {
		return ""
	}
	parts := strings.Fields(value)
	if len(parts) <= 1 {
		return value
	}
	return strings.Join(parts, "-")
}

func visibleTranscript(events []ChatEvent) string {
	var out strings.Builder
	var assistant strings.Builder

	flushAssistant := func() {
		text := strings.TrimSpace(assistant.String())
		if text == "" {
			assistant.Reset()
			return
		}
		out.WriteString("Assistant:\n")
		out.WriteString(text)
		out.WriteString("\n\n")
		assistant.Reset()
	}

	for _, ev := range events {
		switch ev.Type {
		case "user":
			flushAssistant()
			out.WriteString("User:\n")
			out.WriteString(strings.TrimSpace(ev.Text))
			out.WriteString("\n\n")
		case "assistant_text":
			assistant.WriteString(ev.Text)
		case "complete", "error":
			flushAssistant()
		}
	}
	flushAssistant()
	return out.String()
}
