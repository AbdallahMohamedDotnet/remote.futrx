// Package assets publishes embedded container assets and tracks their hashes.
package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/integration/containers/command"
)

const queryTimeout = 10 * time.Second

// Publisher owns the host-to-container publication protocol shared by
// embedded workspace assets. Parent directories remain the responsibility of
// the capability provisioning each asset.
type Publisher struct {
	runner command.Runner
}

// NewPublisher returns a publisher backed by runner.
func NewPublisher(runner command.Runner) *Publisher {
	return &Publisher{runner: runner}
}

// Hash returns the canonical marker value used for published content.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Push publishes content to each destination when its shared marker is stale.
// It is suitable for ordinary workspace assets whose marker is trusted.
func (p *Publisher) Push(
	ctx context.Context,
	containerName string,
	content []byte,
	hashPath string,
	fileMode string,
	destPaths ...string,
) error {
	return p.push(ctx, containerName, content, hashPath, fileMode, false, destPaths...)
}

// PushVerified publishes content like Push, but only treats a current marker as
// authoritative after pulling every destination through LXD and hashing the
// actual bytes on the host. Use it for capability-bearing tooling: both the
// destination and its sidecar marker may be writable from inside the container.
func (p *Publisher) PushVerified(
	ctx context.Context,
	containerName string,
	content []byte,
	hashPath string,
	fileMode string,
	destPaths ...string,
) error {
	return p.push(ctx, containerName, content, hashPath, fileMode, true, destPaths...)
}

func (p *Publisher) push(
	ctx context.Context,
	containerName string,
	content []byte,
	hashPath string,
	fileMode string,
	verifyDestinations bool,
	destPaths ...string,
) error {
	want := Hash(content)

	got, err := command.RunWithTimeout(ctx, p.runner, queryTimeout, "exec", containerName, "--", "cat", hashPath)
	if err == nil &&
		strings.TrimSpace(got) == want &&
		(!verifyDestinations || p.destinationsMatch(ctx, containerName, want, destPaths)) {
		return nil
	}

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	tmp, err := os.CreateTemp("", "futrx-template-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write template: %w", err)
	}
	tmp.Close()

	for _, destPath := range destPaths {
		if out, err := p.runner.Run(pctx, "file", "push", "--mode="+fileMode, tmp.Name(), containerName+destPath); err != nil {
			return fmt.Errorf("push %s: %w; output: %s", destPath, err, out)
		}
	}
	if out, err := p.runner.RunStdin(pctx, strings.NewReader(want), "exec", containerName, "--", "tee", hashPath); err != nil {
		return fmt.Errorf("write %s hash marker: %w; output: %s", hashPath, err, out)
	}
	return nil
}

func (p *Publisher) destinationsMatch(
	ctx context.Context,
	containerName string,
	want string,
	destPaths []string,
) bool {
	if len(destPaths) == 0 {
		return false
	}
	for _, destPath := range destPaths {
		content, err := command.RunWithTimeout(
			ctx,
			p.runner,
			queryTimeout,
			"file",
			"pull",
			containerName+destPath,
			"-",
		)
		if err != nil || Hash([]byte(content)) != want {
			return false
		}
	}
	return true
}
