package main

import (
	"github.com/futrx-com/remote.futrx.com/internal/lifecycle"
	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
	service "github.com/futrx-com/remote.futrx.com/internal/service"
)

// bindLifecycle keeps application-specific subscriber relationships at the
// composition boundary while leaving main focused on startup sequencing.
func bindLifecycle(registry *lifecycle.Registry, services service.Services) (unbind func()) {
	return lifecycle.Bind(registry, lifecycle.Bindings{
		CoreUpdates: []publishers.UpdateSubscriber{
			services.Auth,
		},
	})
}
