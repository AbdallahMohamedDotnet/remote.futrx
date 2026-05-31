package containers

// Container-level env propagation. Used by the user-settings flow to push
// global secrets (Cloudflare token, GitHub PAT, OpenAI key, ...) into every
// project container via LXD's built-in `environment.*` config keys.
//
// LXD exports each `environment.KEY=VALUE` set on the instance into every
// subsequent `lxc exec` session — including the per-prompt
// `lxc exec ... -- claude`. So every CLI inside the container that reads its
// auth from an env var (wrangler, gh, aws, openai, ...) just works without
// any per-tool config file dance.
//
// Already-running processes inside the container keep their old environ
// (Linux semantics); only new exec sessions and child processes pick up the
// new values. That's the unavoidable env-var caveat.

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ApplyContainerEnvDiff sets the values in set and removes the keys in unset,
// using `lxc config set/unset environment.<KEY>`. Idempotent: a set that
// matches the current value is a no-op (reads the current value first to
// avoid a write churn). Returns the first error encountered.
//
// Either map can be nil/empty.
func (m *Manager) ApplyContainerEnvDiff(
	ctx context.Context,
	container string,
	set map[string]string,
	unset []string,
) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	if strings.TrimSpace(container) == "" {
		return errors.New("container name required")
	}

	for k, v := range set {
		qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
		cur, _ := m.lxc.Run(qctx, "config", "get", container, "environment."+k)
		cancelQ()
		if strings.TrimSpace(cur) == v {
			continue
		}
		sctx, cancelS := context.WithTimeout(ctx, queryTimeout)
		out, err := m.lxc.Run(sctx, "config", "set", container, "environment."+k, v)
		cancelS()
		if err != nil {
			return fmt.Errorf("set environment.%s on %s: %w; output: %s", k, container, err, out)
		}
	}

	for _, k := range unset {
		uctx, cancelU := context.WithTimeout(ctx, queryTimeout)
		out, err := m.lxc.Run(uctx, "config", "unset", container, "environment."+k)
		cancelU()
		if err != nil {
			// `lxc config unset` on a missing key prints "not set" but still
			// exits non-zero. Treat that as success.
			if strings.Contains(out, "not set") || strings.Contains(out, "doesn't exist") {
				continue
			}
			return fmt.Errorf("unset environment.%s on %s: %w; output: %s", k, container, err, out)
		}
	}
	return nil
}

// SyncContainerSecrets pushes the full current secrets snapshot (from the
// registered SecretsSource, if any) into the container. Used by Launch to
// seed a freshly created container with the latest values. No-op when no
// source is registered or the snapshot is empty.
func (m *Manager) SyncContainerSecrets(ctx context.Context, container string) error {
	src := m.secretsSnapshot()
	if len(src) == 0 {
		return nil
	}
	return m.ApplyContainerEnvDiff(ctx, container, src, nil)
}
