package containers

// Project skills (and any other agent-shared dotdir contents) live in
// /workspace/.claude/<thing>/... by convention. This file mirrors each
// top-level child of /workspace/.claude into /workspace/.codex as a
// relative symlink, so a skill the user dropped under .claude is
// automatically discoverable by Codex too — without having to maintain
// two copies of the file.
//
// One-way only: .claude -> .codex. Codex-only content (if any) stays in
// .codex untouched; we only add symlinks for entries that don't already
// exist on the codex side.

import (
	"context"
	"errors"
	"time"
)

const ensureWorkspaceSymlinksTimeout = 10 * time.Second

// EnsureWorkspaceClaudeMirror creates symlinks under /workspace/.codex/
// for every top-level entry inside /workspace/.claude/ that doesn't
// already exist on the codex side. Cheap, idempotent, safe to call on
// every prompt: it short-circuits when /workspace/.claude is missing,
// and only adds links — never replaces existing files.
func (m *Manager) EnsureWorkspaceClaudeMirror(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, ensureWorkspaceSymlinksTimeout)
	defer cancel()

	// Shell script runs inside the container as container-root. Workflow:
	//   1. Bail out if /workspace/.claude doesn't exist (nothing to mirror).
	//   2. Create /workspace/.codex if missing.
	//   3. For each top-level child of /workspace/.claude, if the same
	//      name doesn't already exist under /workspace/.codex, create a
	//      relative symlink. Relative so it survives a remount at a
	//      different host path (which doesn't happen today, but cheap
	//      insurance).
	script := `set -eu
src=/workspace/.claude
dst=/workspace/.codex
[ -d "$src" ] || exit 0
mkdir -p "$dst"
for entry in "$src"/*; do
  [ -e "$entry" ] || continue
  name=$(basename "$entry")
  target="$dst/$name"
  if [ ! -e "$target" ] && [ ! -L "$target" ]; then
    ln -s "../.claude/$name" "$target"
  fi
done
`
	if _, err := m.lxc.Run(qctx, "exec", containerName, "--", "sh", "-c", script); err != nil {
		return err
	}
	return nil
}
