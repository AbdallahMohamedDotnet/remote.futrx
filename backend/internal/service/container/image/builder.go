// Package image owns the recipe and workflow for publishing the development
// image consumed by project containers.
package image

// Base-image provisioning. The same install script is used in two places:
//   1. As the recipe baked into the published futrx-remote-dev-base image.
//   2. As the fallback when an already-running container predates Node/npm.
//
// Agent packages supply CLI definitions through provisioning profiles, so
// image builds and runtime repair use the same tested pins as prompt startup.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/shared/output"
)

const (
	// Alias is the image alias the project-container lifecycle launches by
	// default.
	Alias = "futrx-remote-dev-base"

	// SourceImage is the upstream image used as the builder rootfs when
	// (re)building Alias.
	SourceImage = "ubuntu:24.04"

	// baseImageBuilderName is the name used for the throwaway builder
	// container. Kept stable so a retry can clean up a leftover builder
	// from a previous failed run.
	baseImageBuilderName = "futrx-remote-dev-builder"

	baseImageBuildTimeout   = 15 * time.Minute
	baseImagePublishTimeout = 5 * time.Minute
	baseImageNetworkWarmup  = 3 * time.Second
	baseImageProgressTick   = 30 * time.Second
	deleteTimeout           = 30 * time.Second
)

type buildStageResult struct {
	output string
	err    error
}

func runBuildStage(label string, run func() (string, error)) (string, error) {
	started := time.Now()
	log.Printf("%s...", label)

	done := make(chan buildStageResult, 1)
	go func() {
		out, err := run()
		done <- buildStageResult{output: out, err: err}
	}()

	ticker := time.NewTicker(baseImageProgressTick)
	defer ticker.Stop()
	for {
		select {
		case result := <-done:
			elapsed := time.Since(started).Round(time.Second)
			if result.err != nil {
				log.Printf("%s failed after %s", label, elapsed)
			} else {
				log.Printf("%s finished in %s", label, elapsed)
			}
			return result.output, result.err
		case <-ticker.C:
			elapsed := time.Since(started).Round(time.Second)
			log.Printf("%s still running (%s elapsed)", label, elapsed)
		}
	}
}

// Builder owns the disposable builder lifecycle and publishes the
// profile-derived development image consumed by project containers.
type Builder struct {
	runtime                 Runtime
	profiles                ProfileSource
	browserInstallScript    string
	codeServerInstallScript []byte
	networkWarmup           time.Duration
}

// NewBuilder returns an image builder configured with the feature install
// programs layered onto the provider-neutral development image.
func NewBuilder(
	runtime Runtime,
	profileSource ProfileSource,
	browserInstallScript string,
	codeServerInstallScript []byte,
) *Builder {
	return &Builder{
		runtime:                 runtime,
		profiles:                profileSource,
		browserInstallScript:    browserInstallScript,
		codeServerInstallScript: codeServerInstallScript,
		networkWarmup:           baseImageNetworkWarmup,
	}
}

// Build launches a fresh SourceImage container, runs the install script,
// publishes the result under alias, and removes the builder. If alias is
// empty, Alias is used.
//
// Any previous publish at alias is NOT removed automatically. Callers that
// want a clean overwrite must delete the image first.
func (b *Builder) Build(ctx context.Context, alias string) error {
	if !b.runtime.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}
	if alias == "" {
		alias = Alias
	}
	profiles := b.profiles.Snapshot()
	installScript, err := InstallScript(profiles)
	if err != nil {
		return err
	}

	// Clean up any leftover builder from a previous interrupted run before
	// trying to launch a fresh one. Failures are deliberately tolerated.
	cleanCtx, cleanCancel := context.WithTimeout(ctx, deleteTimeout)
	_, _ = b.runtime.DeleteContainer(cleanCtx, baseImageBuilderName)
	cleanCancel()

	bctx, bcancel := context.WithTimeout(ctx, baseImageBuildTimeout)
	defer bcancel()

	// Cleanup deliberately outlives a canceled caller so interrupted builds do
	// not strand their disposable container.
	defer func() {
		dctx, dcancel := context.WithTimeout(context.Background(), deleteTimeout)
		defer dcancel()
		_, _ = b.runtime.DeleteContainer(dctx, baseImageBuilderName)
	}()

	out, err := runBuildStage("[1/6] Downloading Ubuntu 24.04 and starting the builder", func() (string, error) {
		return b.runtime.LaunchContainer(bctx, SourceImage, baseImageBuilderName)
	})
	if err != nil {
		return fmt.Errorf("launch builder: %w; output: %s", err, out)
	}

	// Give cloud-init / systemd-resolved a moment so apt-get can reach the
	// network on the first try.
	select {
	case <-time.After(b.networkWarmup):
	case <-bctx.Done():
		return bctx.Err()
	}

	out, err = runBuildStage("[2/6] Installing system tools, Node.js, and agent CLIs", func() (string, error) {
		return b.runtime.ExecuteScript(bctx, baseImageBuilderName, installScript)
	})
	if err != nil {
		return fmt.Errorf("install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	out, err = runBuildStage("[3/6] Installing the agent browser and Chromium", func() (string, error) {
		return b.runtime.ExecuteScript(bctx, baseImageBuilderName, b.browserInstallScript)
	})
	if err != nil {
		return fmt.Errorf("agent browser install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	out, err = runBuildStage("[4/6] Installing the browser IDE", func() (string, error) {
		return b.runtime.ExecuteScript(bctx, baseImageBuilderName, string(b.codeServerInstallScript))
	})
	if err != nil {
		return fmt.Errorf("code-server install script: %w; output: %s", err, output.Truncate(out, 2000))
	}

	out, err = runBuildStage("[5/6] Finalizing the builder container", func() (string, error) {
		return b.runtime.StopContainer(bctx, baseImageBuilderName)
	})
	if err != nil {
		return fmt.Errorf("stop builder: %w; output: %s", err, out)
	}

	pctx, pcancel := context.WithTimeout(ctx, baseImagePublishTimeout)
	defer pcancel()
	out, err = runBuildStage("[6/6] Publishing the reusable workspace image", func() (string, error) {
		return b.runtime.PublishImage(
			pctx,
			baseImageBuilderName,
			alias,
			description(profiles),
		)
	})
	if err != nil {
		return fmt.Errorf("publish: %w; output: %s", err, out)
	}

	return nil
}
