package auth

import (
	"context"

	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
)

var _ publishers.CoreSubscriber = (*Service)(nil)

// OnUpdateStarted drops 2FA's process-local view before the application is
// replaced. Enrollment records remain in the durable store and are reloaded
// on demand.
func (s *Service) OnUpdateStarted(context.Context, publishers.UpdateStartedEvent) {
	s.twoFactor.invalidateCache()
}

// OnUpdateSucceeded drops any absence that the replacement process may have
// cached while the updater was still settling the installation.
func (s *Service) OnUpdateSucceeded(context.Context, publishers.UpdateSucceededEvent) {
	s.twoFactor.invalidateCache()
}

// OnUpdateFailed follows the same recovery rule. A failed infrastructure
// update can still have restarted the backend or moved durable state before
// rolling back.
func (s *Service) OnUpdateFailed(context.Context, publishers.UpdateFailedEvent) {
	s.twoFactor.invalidateCache()
}
