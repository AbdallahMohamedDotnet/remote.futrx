package image

import (
	"context"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

// Runtime translates the image workflow's semantic operations to a container
// runtime. Command arguments and transport details remain behind this port.
type Runtime interface {
	Available() bool
	DeleteContainer(ctx context.Context, containerName string) (string, error)
	LaunchContainer(ctx context.Context, sourceImage, containerName string) (string, error)
	ExecuteScript(ctx context.Context, containerName, script string) (string, error)
	StopContainer(ctx context.Context, containerName string) (string, error)
	PublishImage(ctx context.Context, containerName, alias, description string) (string, error)
}

// ProfileSource supplies immutable snapshots for recipe generation.
type ProfileSource interface {
	Snapshot() []provisioning.Profile
}
