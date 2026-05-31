package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

func (p *Provider) args(req agent.RunRequest) []string {
	common := []string{
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
	}
	if model := sanitizeModel(req.Model); model != "" {
		common = append(common, "--model", model)
	}
	if req.ResumeID != "" {
		args := append([]string{"exec", "resume"}, common...)
		args = append(args, req.ResumeID, "-")
		return args
	}
	args := append([]string{"exec"}, common...)
	args = append(args, "-")
	return args
}

func sanitizeModel(model string) string {
	model = strings.TrimSpace(model)
	if idx := strings.Index(model, "["); idx > 0 {
		model = strings.TrimSpace(model[:idx])
	}
	return model
}

func (p *Provider) buildCmd(
	ctx context.Context,
	req agent.RunRequest,
	args []string,
	emit func(agent.Event),
) (*exec.Cmd, string, error) {
	cwd := req.Cwd
	if cwd == "" {
		cwd = os.Getenv("HOME")
		if cwd == "" {
			cwd = "/root"
		}
	}

	if req.ProjectID == "" || p.projects == nil {
		cmd := exec.CommandContext(ctx, "codex", args...)
		cmd.Dir = cwd
		cmd.Env = codexEnv(os.Environ())
		cmd.Stdin = strings.NewReader(req.Prompt)
		return cmd, "", nil
	}

	project, err := p.projects.Get(ctx, serviceproject.ID(req.ProjectID))
	if err != nil {
		return nil, "", fmt.Errorf("project not found (%s): %w", req.ProjectID, err)
	}
	if project.ContainerName == "" {
		return nil, "", fmt.Errorf("project %s has no container - recreate the project", project.ID)
	}

	if project.Status != serviceproject.StatusRunning {
		emitSystem(req, emit, "container_starting")
		if _, err := p.projects.Start(ctx, project.ID); err != nil {
			return nil, "", fmt.Errorf("start container: %w", err)
		}
	}

	if p.containers != nil {
		emitSystem(req, emit, "container_preparing")
		if err := p.containers.EnsureCodex(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("install codex in container: %w", err)
		}
		if err := p.containers.EnsureCodexAuth(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("seed codex auth in container: %w", err)
		}
		if err := p.containers.EnsureBootAutostart(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("set container boot.autostart: %w", err)
		}
	}

	lxcArgs := []string{
		"exec",
		"--cwd", "/workspace",
		"--env", "HOME=/root",
		"--env", "CODEX_HOME=/root/.codex",
	}
	if p.projects != nil {
		if secrets, err := p.projects.ListSecrets(ctx, project.ID); err == nil {
			for _, sec := range secrets {
				lxcArgs = append(lxcArgs, "--env", sec.Key+"="+sec.Value)
			}
		}
	}
	lxcArgs = append(lxcArgs, project.ContainerName, "--", "codex")
	lxcArgs = append(lxcArgs, args...)
	cmd := exec.CommandContext(ctx, "lxc", lxcArgs...)
	cmd.Stdin = strings.NewReader(req.Prompt)
	return cmd, project.ContainerName, nil
}

func codexEnv(base []string) []string {
	for _, env := range base {
		if strings.HasPrefix(env, "CODEX_HOME=") {
			return base
		}
	}
	if home := os.Getenv("HOME"); home != "" {
		return append(base, "CODEX_HOME="+home+"/.codex")
	}
	return base
}

func emitSystem(req agent.RunRequest, emit func(agent.Event), subtype string) {
	emit(agent.Event{
		T:              time.Now().UnixMilli(),
		Type:           agent.EventSystem,
		Provider:       agent.ProviderCodex,
		ConversationID: req.ConversationID,
		Subtype:        subtype,
	})
}
