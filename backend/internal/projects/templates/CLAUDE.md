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

## Public URLs for dev servers

Any TCP port you bind to inside the container is reachable from the
public internet at:

```
https://<project-slug>--<port>.dev.remote.futrx.dev
```

Examples — if you start a Vite server on port 5173:

```
$ npm run dev -- --host 0.0.0.0
# → https://<slug>--5173.dev.remote.futrx.dev
```

Notes:

- **Bind to 0.0.0.0**, not 127.0.0.1 — the latter is unreachable from
  the LXD bridge gateway that Caddy forwards through.
- Allowed ports: 1024–65535. Ports below 1024 are blocked.
- Cert is issued on first hit (~5s ACME round trip), then cached.
- Same Google login as the main remote.futrx.dev site — collaborators
  who aren't the admin will be bounced to login. To share an
  unauthenticated URL, you'll need a tunnel like ngrok inside the
  container; that's intentional.
- Same port across different projects is fine: each project's
  container has its own network namespace, so `proj-a` on :3000 and
  `proj-b` on :3000 don't collide.

## Not yet wired

- **Resource limits.** No CPU / memory / disk quotas today — be a
  good neighbor on the shared host.
