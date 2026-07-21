package containers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// templatePublisher owns the host-to-container publication protocol shared by
// embedded workspace assets. Parent directories remain the responsibility of
// the capability provisioning each asset.
type templatePublisher struct {
	lxc CommandRunner
}

// templateHash is the canonical content hash used for the sha256 markers that
// gate template publication and report whether an installed template is current.
func templateHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// push publishes content to each destination when its shared marker is stale.
func (p *templatePublisher) push(
	ctx context.Context,
	containerName string,
	content []byte,
	hashPath string,
	fileMode string,
	destPaths ...string,
) error {
	want := templateHash(content)

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	got, err := p.lxc.Run(qctx, "exec", containerName, "--", "cat", hashPath)
	cancelQ()
	if err == nil && strings.TrimSpace(got) == want {
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
		if out, err := p.lxc.Run(pctx, "file", "push", "--mode="+fileMode, tmp.Name(), containerName+destPath); err != nil {
			return fmt.Errorf("push %s: %w; output: %s", destPath, err, out)
		}
	}
	if out, err := p.lxc.RunStdin(pctx, strings.NewReader(want), "exec", containerName, "--", "tee", hashPath); err != nil {
		return fmt.Errorf("write %s hash marker: %w; output: %s", hashPath, err, out)
	}
	return nil
}
