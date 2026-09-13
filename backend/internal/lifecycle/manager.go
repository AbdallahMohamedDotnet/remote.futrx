// Package lifecycle registers the process-wide lifecycle publishers.
package lifecycle

import "github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"

// Manager is the typed registry of application lifecycle publishers. Event
// definitions, subscriptions, and dispatch stay with each publisher.
type Manager struct {
	Core *publishers.Core
}

// NewManager registers every process-wide lifecycle publisher.
func NewManager() *Manager {
	return &Manager{
		Core: publishers.NewCore(),
	}
}
