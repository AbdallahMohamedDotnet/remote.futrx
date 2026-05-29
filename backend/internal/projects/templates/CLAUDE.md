# Sandbox: remote.futrx.dev

You're running inside an unprivileged LXC container, one per project,
spawned by [remote.futrx.dev](https://remote.futrx.dev). Other
projects can't see this one — fresh `apt` installs, crashed
processes, deleted files only affect you.

## Filesystem

- `/workspace` — your project files. Persistent: bind-mounted from
  the host, survives container stop/start, only removed when the
  project itself is deleted.
- `/root/.claude/` — your auth + session state. Seeded once from
  the host, mutated locally from then on.
- `/root/CLAUDE.md` — this file. Templated and re-pushed by the host
  on every prompt when the template changes, so don't edit it
  expecting changes to stick.
- Everything else: ephemeral. Container reprovision wipes it.

## Capabilities

- You're root inside the container (uid 0). The container is
  unprivileged, so container-root maps to a low-privilege host user —
  no host escape.
- `apt-get install` anything you need. The container is yours to
  reshape.
- Network is fully open: no proxy, no per-project firewall.
- Node 20 + npm are pre-installed (used to install `claude` itself).
- Background processes (e.g. `npm run dev &`) persist in the
  container between your prompts. They die when the container is
  stopped or rebooted.

## Not yet wired

- **Public subdomains per project.** Dev servers you start inside
  the container are not reachable from outside yet. When this lands,
  this file will document the exact host pattern.
- **Resource limits.** No CPU / memory / disk quotas today — be a
  good neighbor on the shared host.
