package prompt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/claudecli"
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
}

type ContainerPreparer interface {
	EnsureClaude(ctx context.Context, containerName string) error
	EnsureClaudeAuth(ctx context.Context, containerName string) error
	EnsureClaudeMD(ctx context.Context, containerName string) error
	EnsureBootAutostart(ctx context.Context, containerName string) error
}

type ClaudeRunner interface {
	Run(
		ctx context.Context,
		id servicechat.ID,
		cmd *exec.Cmd,
		currentSessionID string,
		emit func(ChatEvent),
		updateSession func(sessionID, model string),
	) error
}

type Service struct {
	store      servicechat.Repository
	tmux       TmuxClient
	projects   ProjectResolver // nil = legacy host-only mode
	containers ContainerPreparer
	hub        *runhub.Hub
	claude     ClaudeRunner
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
		store:      store,
		tmux:       tmux,
		projects:   projects,
		containers: containers,
		hub:        hub,
		claude:     claudecli.New(),
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

	// Persist the user message before spawning claude.
	emit(ChatEvent{T: time.Now().UnixMilli(), Type: "user", Text: prompt})

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if meta.Model != "" {
		// Strip context-window suffixes like "[1m]" that may appear in older
		// stored metadata — claude --model doesn't accept those.
		modelArg := meta.Model
		if idx := strings.Index(modelArg, "["); idx > 0 {
			modelArg = modelArg[:idx]
		}
		args = append(args, "--model", modelArg)
	}
	if meta.ClaudeSessionID != "" {
		args = append(args, "--resume", meta.ClaudeSessionID)
	}

	effectivePrompt := promptForMode(meta.Mode, prompt)
	if meta.ClaudeSessionID == "" {
		effectivePrompt = promptWithVisibleHistory(priorEvents, effectivePrompt)
	}

	cmd, err := rnr.buildClaudeCmd(ctx, meta, args, effectivePrompt, cwd, emit)
	if err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: err.Error()})
		return
	}
	err = rnr.claude.Run(ctx, id, cmd, meta.ClaudeSessionID, emit, func(sessionID, model string) {
		_, _ = rnr.store.Update(ctx, id, func(m *ChatMeta) {
			m.ClaudeSessionID = sessionID
			if m.Model == "" && model != "" {
				m.Model = model
			}
		})
	})
	if err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: "claude exit: " + err.Error()})
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

// buildClaudeCmd picks the right spawn target for a chat:
//   - meta.ProjectID empty (or projects backend not wired) → run on the host
//     the way we always have: claude binary at the chat's cwd.
//   - meta.ProjectID set → run inside the project's LXC container via
//     `lxc exec --cwd /workspace -- claude …`. The container's /workspace is
//     a bind-mount of the host's project workspace dir; claude operates on
//     it natively, never seeing the host filesystem outside the bind mounts.
//
// On the project path we also auto-start a stopped container and lazily
// install the claude CLI on first prompt (one-time per project, ~60s).
func (rnr *Service) buildClaudeCmd(
	ctx context.Context,
	meta ChatMeta,
	args []string,
	prompt string,
	hostCwd string,
	emit func(ChatEvent),
) (*exec.Cmd, error) {
	if meta.ProjectID == "" || rnr.projects == nil {
		cmd := exec.CommandContext(ctx, "claude", args...)
		cmd.Dir = hostCwd
		// IS_SANDBOX=1 lets `claude --dangerously-skip-permissions` run under
		// uid 0. The box is single-user and the UI is auto-approve.
		cmd.Env = append(os.Environ(), "IS_SANDBOX=1")
		cmd.Stdin = strings.NewReader(prompt)
		return cmd, nil
	}

	p, err := rnr.projects.Get(ctx, serviceproject.ID(meta.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("project not found (%s): %w", meta.ProjectID, err)
	}
	if p.ContainerName == "" {
		return nil, fmt.Errorf("project %s has no container — recreate the project", p.ID)
	}

	// Auto-start if the container isn't running. Surface progress to the
	// client so the user sees what's happening (cold-start can be 5–10s).
	if p.Status != serviceproject.StatusRunning {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "system", Subtype: "container_starting"})
		if _, err := rnr.projects.Start(ctx, p.ID); err != nil {
			return nil, fmt.Errorf("start container: %w", err)
		}
	}

	// Lazy claude install + auth refresh. Install is ~60s once per project;
	// auth only pushes when the host login files are newer than the container copy.
	if rnr.containers != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "system", Subtype: "container_preparing"})
		if err := rnr.containers.EnsureClaude(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("install claude in container: %w", err)
		}
		if err := rnr.containers.EnsureClaudeAuth(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("seed claude auth in container: %w", err)
		}
		if err := rnr.containers.EnsureClaudeMD(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("push CLAUDE.md to container: %w", err)
		}
		if err := rnr.containers.EnsureBootAutostart(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("set container boot.autostart: %w", err)
		}
	}

	lxcArgs := []string{
		"exec",
		"--cwd", "/workspace",
		"--env", "IS_SANDBOX=1",
		"--env", "HOME=/root",
		p.ContainerName,
		"--",
		"claude",
	}
	lxcArgs = append(lxcArgs, args...)
	cmd := exec.CommandContext(ctx, "lxc", lxcArgs...)
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, nil
}
