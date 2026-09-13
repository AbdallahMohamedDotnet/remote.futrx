package publishers

import "context"

// UpdateStartedEvent describes an application self-update that has started.
type UpdateStartedEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// UpdateSucceededEvent describes an application self-update that completed
// successfully. It is intentionally the same small, non-sensitive identity as
// the start event; subscribers that need durable details own their own state.
type UpdateSucceededEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// UpdateFailedEvent describes an application self-update that terminated
// unsuccessfully.
type UpdateFailedEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// UpdateSubscriber receives application self-update lifecycle events.
type UpdateSubscriber interface {
	OnUpdateStarted(context.Context, UpdateStartedEvent)
	OnUpdateSucceeded(context.Context, UpdateSucceededEvent)
	OnUpdateFailed(context.Context, UpdateFailedEvent)
}
