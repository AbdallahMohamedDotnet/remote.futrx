package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
)

type bindingTestStatus struct {
	Authenticated bool        `json:"authenticated"`
	DeviceLogin   DeviceState `json:"deviceLogin"`
}

func TestDeviceBindingPreservesConcreteStatus(t *testing.T) {
	service := NewDeviceService(DeviceConfig[bindingTestStatus]{
		BuildStatus: func() DeviceStatusBuilder[bindingTestStatus] {
			return func(state DeviceState) bindingTestStatus {
				return bindingTestStatus{DeviceLogin: state}
			}
		},
	})
	binding := NewDeviceBinding(agent.ProviderCodex, service)

	if binding.ID() != agent.ProviderCodex || binding.Flow() != FlowDevice {
		t.Fatalf("binding identity = (%q, %q)", binding.ID(), binding.Flow())
	}
	if _, ok := binding.Status().(bindingTestStatus); !ok {
		t.Fatalf("status type = %T, want bindingTestStatus", binding.Status())
	}

	subscription, err := binding.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer subscription.Close()

	status, ok := subscription.Next(context.Background())
	if !ok {
		t.Fatal("initial status was not delivered")
	}
	if _, ok := status.(bindingTestStatus); !ok {
		t.Fatalf("streamed status type = %T, want bindingTestStatus", status)
	}
}

func TestCodeBindingClassifiesConfiguredInputErrors(t *testing.T) {
	required := errors.New("code required")
	noSession := errors.New("no session")
	binding := NewCodeBinding(agent.ProviderClaude, NewCodeService(CodeConfig{
		CodeRequired: required,
		NoSession:    noSession,
	}))

	for _, err := range []error{required, fmt.Errorf("wrapped: %w", noSession)} {
		if !binding.IsCodeInputError(err) {
			t.Fatalf("IsCodeInputError(%v) = false", err)
		}
	}
	if binding.IsCodeInputError(errors.New("internal")) {
		t.Fatal("unexpected error classified as caller input")
	}
}

func TestUnavailableBindingRejectsSubscription(t *testing.T) {
	binding := NewCodeBinding(agent.ProviderClaude, nil)
	if binding.Available() {
		t.Fatal("nil service binding is available")
	}
	if _, err := binding.Subscribe(); !errors.Is(err, ErrUnsupportedFlow) {
		t.Fatalf("Subscribe error = %v, want ErrUnsupportedFlow", err)
	}
}
