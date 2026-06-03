package containers

// Project skills live in /workspace/.agents/skills. Claude and legacy Codex
// locations are compatibility links so users only edit one source of truth.

import (
	"context"
	"errors"
	"time"
)

const ensureWorkspaceSymlinksTimeout = 10 * time.Second

// EnsureWorkspaceSkillLinks creates the canonical .agents skills directory,
// migrates legacy .claude/.codex skill children when possible, and points both
// compatibility paths at .agents/skills. Cheap and idempotent, so providers can
// call it before every prompt.
func (m *Manager) EnsureWorkspaceSkillLinks(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, ensureWorkspaceSymlinksTimeout)
	defer cancel()

	script := `set -eu
canonical=/workspace/.agents/skills
mkdir -p /workspace/.agents "$canonical" /workspace/.claude /workspace/.codex
chmod 755 /workspace/.agents "$canonical" /workspace/.claude /workspace/.codex

migrate_skills_dir() {
  src="$1"
  [ -e "$src" ] || return 0
  [ ! -L "$src" ] || return 0
  [ -d "$src" ] || return 0

  for entry in "$src"/* "$src"/.[!.]* "$src"/..?*; do
    [ -e "$entry" ] || continue
    name=$(basename "$entry")
    [ "$name" != "." ] && [ "$name" != ".." ] || continue
    target="$canonical/$name"
    if [ ! -e "$target" ] && [ ! -L "$target" ]; then
      mv "$entry" "$target"
    fi
  done
  rmdir "$src" 2>/dev/null || true
}

link_skills_dir() {
  base="$1"
  link="$base/skills"
  target="../.agents/skills"
  if [ -L "$link" ]; then
    current=$(readlink "$link")
    if [ "$current" != "$target" ]; then
      rm "$link"
      ln -s "$target" "$link"
    fi
  elif [ ! -e "$link" ]; then
    ln -s "$target" "$link"
  fi
}

migrate_skills_dir /workspace/.claude/skills
migrate_skills_dir /workspace/.codex/skills
link_skills_dir /workspace/.claude
link_skills_dir /workspace/.codex
`
	if _, err := m.lxc.Run(qctx, "exec", containerName, "--", "sh", "-c", script); err != nil {
		return err
	}
	return nil
}
