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
- `/root/.claude/CLAUDE.md` — this file. Loaded by claude as
  user-level memory on every session. Templated and re-pushed by
  the host on every prompt when the template changes, so don't edit
  it expecting changes to stick.
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

### Dev-server Host-header allowlists (per-framework)

Caddy preserves the public host (`<slug>--<port>.dev.remote.futrx.dev`)
when proxying — that's correct (so frameworks generate the right
absolute URLs for OAuth, assets, etc.) but several dev servers have
a Host allowlist for DNS-rebinding/CSRF defense and refuse it. When
the user asks you to start a dev server, apply the right fix below
as part of the same change so the public URL works first try.

**Just works, no config needed** — start them and the public URL
serves immediately:

- `php -S 0.0.0.0:<port>`, Laravel `artisan serve`, Symfony local
- `python -m http.server`, Flask, FastAPI, Uvicorn
- Plain Node `http.createServer`, Go `net/http`, Rust hyper, etc.

**Has a Host allowlist, widen it**:

- **Vite** — `vite.config.{js,ts}`:

  ```js
  server: { host: true, allowedHosts: ['.dev.remote.futrx.dev'] }
  ```

- **Next.js 13+ dev** — `next.config.{js,ts}`:

  ```js
  experimental: { allowedDevOrigins: ['*.dev.remote.futrx.dev'] }
  ```

- **Webpack dev-server** — `webpack.config.js`:

  ```js
  devServer: { allowedHosts: 'all', host: '0.0.0.0' }
  ```

- **Create React App** — env: `DANGEROUSLY_DISABLE_HOST_CHECK=true`

- **Angular CLI** — `ng serve --host 0.0.0.0 --allowed-hosts '.dev.remote.futrx.dev'`

- **Django** — `settings.py`:

  ```py
  ALLOWED_HOSTS = ['.dev.remote.futrx.dev', 'localhost']
  ```

- **Rails 6+** — `config/environments/development.rb`:

  ```rb
  config.hosts << '.dev.remote.futrx.dev'
  ```

Generic rule when configuring a stack you don't recognize: search
its docs for "allowed hosts" / "host allowlist" / "disable host
check" and either add `.dev.remote.futrx.dev` (with the leading dot
to cover all subdomains) or set the "allow all" knob if available.

## Not yet wired

- **Resource limits.** No CPU / memory / disk quotas today — be a
  good neighbor on the shared host.
