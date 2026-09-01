package workspace

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const ensureRuntimeAssetsTimeout = 30 * time.Second

// EnsureRuntimeAssets publishes the selected provider's non-secret runtime
// configuration. Content is declared by the provider profile and verified on
// every preparation because both the asset and marker live in a root-writable
// provider home.
func (p *Provisioner) EnsureRuntimeAssets(
	ctx context.Context,
	containerName string,
	templates []provisioning.TemplateFile,
) error {
	if len(templates) == 0 {
		return nil
	}
	if !p.runner.Available() {
		return command.ErrUnavailable
	}

	dctx, cancel := context.WithTimeout(ctx, ensureRuntimeAssetsTimeout)
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
			out, err := p.runner.Run(dctx, "exec", containerName, "--",
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
		if err := p.publisher.PushVerified(
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
