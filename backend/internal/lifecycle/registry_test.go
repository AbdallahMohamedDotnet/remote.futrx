package lifecycle

import "testing"

func TestRegistryContainsCorePublisher(t *testing.T) {
	registry := NewRegistry()
	if registry.Core == nil {
		t.Fatal("core publisher is not registered")
	}
}
