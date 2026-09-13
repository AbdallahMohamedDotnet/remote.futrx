// Package lifecycle registers the process-wide lifecycle publishers and binds
// application subscribers to them.
package lifecycle

import "github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"

// Registry exposes the process-wide lifecycle publishers. Event definitions,
// subscriptions, and dispatch stay with each publisher.
type Registry struct {
	Core *publishers.Core
}

// NewRegistry constructs every process-wide lifecycle publisher.
func NewRegistry() *Registry {
	return &Registry{
		Core: publishers.NewCore(),
	}
}
