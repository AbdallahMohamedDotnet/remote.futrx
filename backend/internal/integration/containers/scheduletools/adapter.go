// Package scheduletools provisions the workspace skill and CLI used by agents
// to manage Remote-owned scheduled tasks.
package scheduletools

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/assets"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const (
	containerScriptsDir        = "/workspace/scripts"
	containerScheduleCLI       = containerScriptsDir + "/remote-schedule"
	containerScheduleCLIHash   = containerScriptsDir + "/.remote-schedule.sha256"
	containerScheduleSkillDir  = "/workspace/.agents/skills/scheduled-tasks"
	containerScheduleSkillMD   = containerScheduleSkillDir + "/SKILL.md"
	containerScheduleSkillHash = containerScheduleSkillDir + "/.skill.sha256"

	ensureTimeout = 10 * time.Second
)

//go:embed assets/remote-schedule
var scheduleCLI []byte

//go:embed assets/skills/scheduled-tasks/SKILL.md
var scheduleSkill []byte

// Adapter publishes the provider-neutral scheduled-task tooling into project
// containers. The assets live under /workspace so they survive replacement of
// the project container.
type Adapter struct {
	runner    command.Runner
	publisher *assets.Publisher
}

// NewAdapter returns scheduled-task tooling backed by the shared container
// runner and asset publisher.
func NewAdapter(runner command.Runner, publisher *assets.Publisher) *Adapter {
	return &Adapter{
		runner:    runner,
		publisher: publisher,
	}
}

// Ensure idempotently provisions the remote-schedule executable and its skill.
func (a *Adapter) Ensure(ctx context.Context, containerName string) error {
	if !a.runner.Available() {
		return command.ErrUnavailable
	}

	out, err := command.RunWithTimeout(
		ctx,
		a.runner,
		ensureTimeout,
		"exec",
		containerName,
		"--",
		"install",
		"-d",
		"-m",
		"755",
		containerScriptsDir,
		containerScheduleSkillDir,
	)
	if err != nil {
		return fmt.Errorf("create scheduled-task tooling directories: %w; output: %s", err, out)
	}

	if err := a.publisher.PushVerified(
		ctx,
		containerName,
		scheduleCLI,
		containerScheduleCLIHash,
		"755",
		containerScheduleCLI,
	); err != nil {
		return fmt.Errorf("publish remote-schedule: %w", err)
	}

	if err := a.publisher.PushVerified(
		ctx,
		containerName,
		scheduleSkill,
		containerScheduleSkillHash,
		"644",
		containerScheduleSkillMD,
	); err != nil {
		return fmt.Errorf("publish scheduled-tasks skill: %w", err)
	}
	return nil
}
