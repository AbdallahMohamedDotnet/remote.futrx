package claude

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

// browserMCPConfigPath is the --mcp-config file claude loads when the
// browser skill is active. Must match containers.ContainerMCPClaudeConfig.
const browserMCPConfigPath = "/workspace/.browser-gui/mcp-claude.json"

func (p *Provider) args(req agent.RunRequest) []string {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if model := sanitizeModel(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if req.ResumeID != "" {
		args = append(args, "--resume", req.ResumeID)
		if req.Fork {
			args = append(args, "--fork-session")
		}
	}
	if req.EnableBrowser {
		args = append(args, "--mcp-config", browserMCPConfigPath)
	}
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
		cmd := exec.CommandContext(ctx, "claude", args...)
		cmd.Dir = cwd
		// IS_SANDBOX=1 lets `claude --dangerously-skip-permissions` run under
		// uid 0. The box is single-user and the UI is auto-approve.
		cmd.Env = append(os.Environ(), "IS_SANDBOX=1")
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
		if err := p.containers.EnsureClaude(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("install claude in container: %w", err)
		}
		if err := p.containers.EnsureClaudeAuth(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("seed claude auth in container: %w", err)
		}
		if err := p.containers.EnsureAgentInstructions(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("push agent instructions to container: %w", err)
		}
		if err := p.containers.EnsureWorkspaceSkillLinks(ctx, project.ContainerName); err != nil {
			// Symlink shim is best-effort: a stale /workspace/.codex shouldn't
			// block a claude run that doesn't depend on it.
			_ = err
		}
		if err := p.containers.EnsureBrowserScript(ctx, project.ContainerName); err != nil {
			// Browser script + config are best-effort: their absence only matters
			// when the agent tries to drive Playwright. Don't fail the run.
			_ = err
		}
		if req.EnableBrowser {
			if err := p.containers.EnsureBrowserMCP(ctx, project.ContainerName); err != nil {
				return nil, "", fmt.Errorf("provision browser MCP: %w", err)
			}
			if err := p.containers.EnsureBrowserGUICore(ctx, project.ContainerName); err != nil {
				return nil, "", fmt.Errorf("start browser core: %w", err)
			}
		}
		if err := p.containers.EnsureBootAutostart(ctx, project.ContainerName); err != nil {
			return nil, "", fmt.Errorf("set container boot.autostart: %w", err)
		}
	}

	lxcArgs := []string{
		"exec",
		"--cwd", "/workspace",
		"--env", "IS_SANDBOX=1",
		"--env", "HOME=/root",
	}
	if p.projects != nil {
		if secrets, err := p.projects.ListSecrets(ctx, project.ID); err == nil {
			for _, sec := range secrets {
				lxcArgs = append(lxcArgs, "--env", sec.Key+"="+sec.Value)
			}
		}
	}
	lxcArgs = append(lxcArgs, project.ContainerName, "--", "claude")
	lxcArgs = append(lxcArgs, args...)
	cmd := exec.CommandContext(ctx, "lxc", lxcArgs...)
	cmd.Stdin = strings.NewReader(req.Prompt)
	return cmd, project.ContainerName, nil
}

func emitSystem(req agent.RunRequest, emit func(agent.Event), subtype string) {
	emit(agent.Event{
		T:              time.Now().UnixMilli(),
		Type:           agent.EventSystem,
		Provider:       agent.ProviderClaude,
		ConversationID: req.ConversationID,
		Subtype:        subtype,
	})
}
