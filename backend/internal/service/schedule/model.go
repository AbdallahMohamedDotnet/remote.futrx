package schedule

import (
	"strings"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
)

// ID is the stable identifier of a scheduled task.
type ID string

type Kind string

const (
	KindOnce Kind = "once"
	KindCron Kind = "cron"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusExhausted Status = "exhausted"
	StatusError     Status = "error"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusSkipped   RunStatus = "skipped"
	RunStatusQueued    RunStatus = "queued"
	RunStatusAbandoned RunStatus = "abandoned"
)

type OverlapPolicy string

const (
	// OverlapQueueOne coalesces any number of overlapping occurrences into
	// one follow-up run.
	OverlapQueueOne OverlapPolicy = "queue_one"
	// OverlapSkip records and consumes an occurrence when its chat is busy.
	OverlapSkip OverlapPolicy = "skip"
)

// Task is both the domain model and the JSON persistence/API representation.
// Times are Unix milliseconds.
type Task struct {
	ID         ID                `json:"id"`
	Name       string            `json:"name"`
	OwnerEmail string            `json:"ownerEmail"`
	ProjectID  serviceproject.ID `json:"projectId"`
	ChatID     servicechat.ID    `json:"chatId"`
	Prompt     string            `json:"prompt"`
	Kind       Kind              `json:"kind"`
	At         int64             `json:"at,omitempty"`
	Cron       string            `json:"cron,omitempty"`
	Timezone   string            `json:"timezone"`
	Enabled    bool              `json:"enabled"`
	Status     Status            `json:"status"`
	NextRunAt  int64             `json:"nextRunAt,omitempty"`
	RunCount   int               `json:"runCount"`
	MaxRuns    int               `json:"maxRuns,omitempty"`
	Overlap    OverlapPolicy     `json:"overlapPolicy,omitempty"`
	LastRunAt  int64             `json:"lastRunAt,omitempty"`
	LastRunEnd int64             `json:"lastRunFinishedAt,omitempty"`
	LastStatus RunStatus         `json:"lastRunStatus,omitempty"`
	LastError  string            `json:"lastError,omitempty"`
	CreatedAt  int64             `json:"createdAt"`
	UpdatedAt  int64             `json:"updatedAt"`

	// Claim fields are persisted before an agent is started. They make a
	// process crash observable and prevent duplicate dispatch within a
	// running process.
	ActiveRunID      string `json:"activeRunId,omitempty"`
	ActiveRunStarted int64  `json:"activeRunStartedAt,omitempty"`
	ActiveRunForced  bool   `json:"activeRunForced,omitempty"`
	PendingRun       bool   `json:"pendingRun,omitempty"`
	PendingRunForced bool   `json:"pendingRunForced,omitempty"`
	PendingSince     int64  `json:"pendingSince,omitempty"`
	RetryAt          int64  `json:"retryAt,omitempty"`
}

type CreateInput struct {
	Name      string            `json:"name"`
	ProjectID serviceproject.ID `json:"projectId"`
	ChatID    servicechat.ID    `json:"chatId"`
	Prompt    string            `json:"prompt"`
	Kind      Kind              `json:"kind"`
	At        int64             `json:"at,omitempty"`
	Cron      string            `json:"cron,omitempty"`
	Timezone  string            `json:"timezone"`
	MaxRuns   int               `json:"maxRuns,omitempty"`
	Overlap   OverlapPolicy     `json:"overlapPolicy,omitempty"`
}

type UpdateInput struct {
	Name     *string        `json:"name,omitempty"`
	Prompt   *string        `json:"prompt,omitempty"`
	Kind     *Kind          `json:"kind,omitempty"`
	At       *int64         `json:"at,omitempty"`
	Cron     *string        `json:"cron,omitempty"`
	Timezone *string        `json:"timezone,omitempty"`
	MaxRuns  *int           `json:"maxRuns,omitempty"`
	Overlap  *OverlapPolicy `json:"overlapPolicy,omitempty"`
	Enabled  *bool          `json:"enabled,omitempty"`
}

func ValidID(id ID) bool {
	if len(id) != 24 {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
