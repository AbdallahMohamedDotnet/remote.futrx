package clauderuntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/claudecli"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
)

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

type ProcessRunner interface {
	Run(
		ctx context.Context,
		id servicechat.ID,
		cmd *exec.Cmd,
		currentSessionID string,
		emit func(prompt.ChatEvent),
		updateSession func(sessionID, model string),
	) error
}

type Runtime struct {
	projects   ProjectResolver
	containers ContainerPreparer
	claude     ProcessRunner
}

func New(projects ProjectResolver, containers ContainerPreparer) *Runtime {
	return NewWithRunner(projects, containers, claudecli.New())
}

func NewWithRunner(projects ProjectResolver, containers ContainerPreparer, claude ProcessRunner) *Runtime {
	if claude == nil {
		claude = claudecli.New()
	}
	return &Runtime{
		projects:   projects,
		containers: containers,
		claude:     claude,
	}
}

func (r *Runtime) Run(
	ctx context.Context,
	req prompt.ClaudeRunRequest,
	emit func(prompt.ChatEvent),
	updateSession func(sessionID, model string),
) error {
	cmd, err := r.buildCommand(ctx, req, emit)
	if err != nil {
		return err
	}
	return r.claude.Run(ctx, req.ChatID, cmd, req.CurrentSessionID, emit, updateSession)
}

// buildCommand picks the right spawn target for a chat:
//   - ProjectID empty, or no project backend wired: run the host claude binary
//     at the chat cwd.
//   - ProjectID set: run claude inside the project's LXC container through
//     `lxc exec --cwd /workspace -- claude ...`.
func (r *Runtime) buildCommand(
	ctx context.Context,
	req prompt.ClaudeRunRequest,
	emit func(prompt.ChatEvent),
) (*exec.Cmd, error) {
	if req.ProjectID == "" || r.projects == nil {
		cmd := exec.CommandContext(ctx, "claude", req.Args...)
		cmd.Dir = req.HostCwd
		// IS_SANDBOX=1 lets `claude --dangerously-skip-permissions` run under
		// uid 0. The box is single-user and the UI is auto-approve.
		cmd.Env = append(os.Environ(), "IS_SANDBOX=1")
		cmd.Stdin = strings.NewReader(req.Prompt)
		return cmd, nil
	}

	p, err := r.projects.Get(ctx, serviceproject.ID(req.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("project not found (%s): %w", req.ProjectID, err)
	}
	if p.ContainerName == "" {
		return nil, fmt.Errorf("project %s has no container - recreate the project", p.ID)
	}

	if p.Status != serviceproject.StatusRunning {
		emit(prompt.ChatEvent{T: time.Now().UnixMilli(), Type: "system", Subtype: "container_starting"})
		if _, err := r.projects.Start(ctx, p.ID); err != nil {
			return nil, fmt.Errorf("start container: %w", err)
		}
	}

	if r.containers != nil {
		emit(prompt.ChatEvent{T: time.Now().UnixMilli(), Type: "system", Subtype: "container_preparing"})
		if err := r.containers.EnsureClaude(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("install claude in container: %w", err)
		}
		if err := r.containers.EnsureClaudeAuth(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("seed claude auth in container: %w", err)
		}
		if err := r.containers.EnsureClaudeMD(ctx, p.ContainerName); err != nil {
			return nil, fmt.Errorf("push CLAUDE.md to container: %w", err)
		}
		if err := r.containers.EnsureBootAutostart(ctx, p.ContainerName); err != nil {
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
	lxcArgs = append(lxcArgs, req.Args...)
	cmd := exec.CommandContext(ctx, "lxc", lxcArgs...)
	cmd.Stdin = strings.NewReader(req.Prompt)
	return cmd, nil
}
