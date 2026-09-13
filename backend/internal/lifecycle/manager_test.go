package lifecycle

import "testing"

func TestManagerRegistersCorePublisher(t *testing.T) {
	manager := NewManager()
	if manager.Core == nil {
		t.Fatal("core publisher is not registered")
	}
}
