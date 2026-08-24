package chat

import "context"

type suppressEventNotificationsKey struct{}

// SuppressEventNotifications marks repository writes that copy existing
// history rather than recording a new user-visible event. Persistence and
// workspace updates still happen; notification side effects do not.
func SuppressEventNotifications(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressEventNotificationsKey{}, true)
}

// EventNotificationsSuppressed reports whether an append is historical replay.
func EventNotificationsSuppressed(ctx context.Context) bool {
	suppressed, _ := ctx.Value(suppressEventNotificationsKey{}).(bool)
	return suppressed
}
