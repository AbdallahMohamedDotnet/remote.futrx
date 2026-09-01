// Package runtimeassets publishes provider-selected runtime templates inside
// project containers.
package runtimeassets

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/assets"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const ensureTimeout = 30 * time.Second

// Adapter publishes the selected provider's non-secret runtime configuration.
// Content is declared by the provider profile and verified on every
// preparation because both the asset and marker live in a root-writable
// provider home.
type Adapter struct {
	runner    command.Runner
	publisher *assets.Publisher
}

// NewAdapter returns a runtime-asset adapter backed by shared container
// dependencies.
func NewAdapter(runner command.Runner, publisher *assets.Publisher) *Adapter {
	return &Adapter{runner: runner, publisher: publisher}
}

// Ensure publishes every selected template to the project container.
func (a *Adapter) Ensure(
	ctx context.Context,
	containerName string,
	templates []provisioning.TemplateFile,
) error {
	if len(templates) == 0 {
		return nil
	}
	if !a.runner.Available() {
		return command.ErrUnavailable
	}

	dctx, cancel := context.WithTimeout(ctx, ensureTimeout)
	defer cancel()
	created := make(map[string]bool, len(templates))
	for _, template := range templates {
		directory := template.Directory
		if directory == "" {
			directory = path.Dir(template.Path)
		}
		if !created[directory] {
			directoryMode := template.DirectoryMode
			if directoryMode == "" {
				directoryMode = "700"
			}
			out, err := a.runner.Run(dctx, "exec", containerName, "--",
				"install", "-d", "-m", directoryMode, directory)
			if err != nil {
				return fmt.Errorf("mkdir %s: %w; output: %s", directory, err, out)
			}
			created[directory] = true
		}

		mode := template.Mode
		if mode == "" {
			mode = "644"
		}
		if err := a.publisher.PushVerified(
			ctx,
			containerName,
			template.Content,
			template.HashPath,
			mode,
			template.Path,
		); err != nil {
			return err
		}
	}
	return nil
}
