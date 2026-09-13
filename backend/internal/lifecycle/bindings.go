package lifecycle

import "github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"

// Bindings groups application subscribers by the publisher whose events they
// consume. Concrete service selection remains in the process composition root.
type Bindings struct {
	Core []publishers.CoreSubscriber
}

// Bind registers every subscriber and returns an idempotent cleanup function.
func Bind(registry *Registry, bindings Bindings) (unbind func()) {
	unsubscribes := make([]func(), 0, len(bindings.Core))
	for _, subscriber := range bindings.Core {
		unsubscribes = append(unsubscribes, registry.Core.Subscribe(subscriber))
	}

	return func() {
		for _, unsubscribe := range unsubscribes {
			unsubscribe()
		}
	}
}
