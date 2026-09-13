package lifecycle

import (
	"context"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
)

type recordingUpdateSubscriber struct {
	started int
}

func (s *recordingUpdateSubscriber) OnUpdateStarted(context.Context, publishers.UpdateStartedEvent) {
	s.started++
}

func (*recordingUpdateSubscriber) OnUpdateSucceeded(context.Context, publishers.UpdateSucceededEvent) {
}

func (*recordingUpdateSubscriber) OnUpdateFailed(context.Context, publishers.UpdateFailedEvent) {}

func TestBindRegistersAndUnregistersCoreUpdateSubscribers(t *testing.T) {
	registry := NewRegistry()
	first := &recordingUpdateSubscriber{}
	second := &recordingUpdateSubscriber{}
	unbind := Bind(registry, Bindings{
		CoreUpdates: []publishers.UpdateSubscriber{first, second},
	})

	registry.Core.PublishUpdateStarted(context.Background(), "0.4.0", "application", "admin@example.com")
	if first.started != 1 || second.started != 1 {
		t.Fatalf("started deliveries = (%d, %d), want (1, 1)", first.started, second.started)
	}

	unbind()
	unbind()
	registry.Core.PublishUpdateStarted(context.Background(), "0.5.0", "application", "admin@example.com")
	if first.started != 1 || second.started != 1 {
		t.Fatalf("deliveries after unbind = (%d, %d), want (1, 1)", first.started, second.started)
	}
}
