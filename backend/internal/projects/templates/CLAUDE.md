# Sandbox: remote.futrx.dev

You're running inside an unprivileged LXC container, one per project,
spawned by [remote.futrx.dev](https://remote.futrx.dev). Other
projects can't see this one — fresh `apt` installs, crashed
processes, deleted files only affect you.

## Filesystem

- `/workspace` — your project files. Persistent, survives container
  restarts and reprovisions.
- `/root/.claude/CLAUDE.md` — this file. The host re-pushes it when
  the template changes; don't edit it expecting changes to stick.
- Everything else: ephemeral.

## Capabilities

- Root in the container (uid 0). Unprivileged means container-root
  maps to a low-privilege host user — no host escape.
- `apt-get install` whatever you need.
- Network is fully open.
- Node 20 + npm are pre-installed.
- Background processes persist between prompts; they die on container
  stop/reboot.

## Dev servers — there is no localhost, there is a public URL

Whenever the user asks for a dev server, **the URL they reach it at
is**:

```
https://<this-project-slug>--<port>.dev.remote.futrx.dev
```

Replace `<this-project-slug>` with this project's slug and `<port>`
with whatever you bound. Pretend `localhost:<port>` doesn't exist
for any user-facing purpose — they're not on this box, they're on
the public internet. Cert is auto-issued on first hit.

Two rules to make it work:

1. **Bind to `0.0.0.0`, not `127.0.0.1`** (otherwise the LXD bridge
   can't reach it).
2. **If your dev server has a Host allowlist, add
   `.dev.remote.futrx.dev` to it.** This bites Vite, Next.js dev,
   Webpack, CRA, Angular, Django (`ALLOWED_HOSTS`), Rails
   (`config.hosts`). It does NOT bite `php -S`, plain Python/Node/Go
   servers, Flask, FastAPI, Symfony, Laravel. When in doubt: search
   your framework's docs for "allowed hosts" and widen it.

After you start a server, tell the user the public URL, not
`http://localhost:<port>`.
