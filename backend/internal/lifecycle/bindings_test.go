package lifecycle

import (
	"context"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
)

type recordingCoreSubscriber struct {
	started int
}

func (s *recordingCoreSubscriber) OnUpdateStarted(context.Context, publishers.UpdateStartedEvent) {
	s.started++
}

func (*recordingCoreSubscriber) OnUpdateSucceeded(context.Context, publishers.UpdateSucceededEvent) {}

func (*recordingCoreSubscriber) OnUpdateFailed(context.Context, publishers.UpdateFailedEvent) {}

func TestBindRegistersAndUnregistersCoreSubscribers(t *testing.T) {
	registry := NewRegistry()
	first := &recordingCoreSubscriber{}
	second := &recordingCoreSubscriber{}
	unbind := Bind(registry, Bindings{
		Core: []publishers.CoreSubscriber{first, second},
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
