package agent

import "context"

// ProjectID is the provider-facing identity of a Remote project. It is kept
// independent from the project service's storage and transport models so agent
// modules depend only on this narrow execution port.
type ProjectID string

type ProjectStatus string

const ProjectStatusRunning ProjectStatus = "running"

// Project contains only the project state an agent needs to prepare and run a
// CLI inside its workspace container.
type Project struct {
	ID            ProjectID
	ContainerName string
	Status        ProjectStatus
}

// ProjectSecret is an environment variable made available to an agent run.
type ProjectSecret struct {
	Key   string
	Value string
}

// ProjectResolver is the complete project surface available to agent
// adapters. Service-layer project models are translated at the composition
// boundary and never leak into provider packages.
type ProjectResolver interface {
	Get(context.Context, ProjectID) (Project, error)
	Start(context.Context, ProjectID) (Project, error)
	ListSecrets(context.Context, ProjectID) ([]ProjectSecret, error)
}
