package auth

import (
	"context"

	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
)

var _ publishers.UpdateSubscriber = (*Service)(nil)

// OnUpdateStarted is the 2FA hook for application-update preparation. The
// file-backed 2FA store already persists enrollment across process restarts,
// so it currently requires no preparation step.
func (*Service) OnUpdateStarted(context.Context, publishers.UpdateStartedEvent) {}

// OnUpdateSucceeded is the 2FA hook in the replacement process. The store is
// initialized during normal startup and needs no post-update mutation.
func (*Service) OnUpdateSucceeded(context.Context, publishers.UpdateSucceededEvent) {}

// OnUpdateFailed is the 2FA hook for update rollback or recovery. Durable 2FA
// records are not changed by the updater, so no compensating action is needed.
func (*Service) OnUpdateFailed(context.Context, publishers.UpdateFailedEvent) {}
