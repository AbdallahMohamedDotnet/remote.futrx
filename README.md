# remote.futrx.dev

A self-hosted, mobile-first **Claude Code chat UI** with a Go backend that drives the official `claude` CLI in streaming mode. Each "project" in the sidebar is its own unprivileged LXC container; dev servers inside any project are reachable at `<slug>--<port>.dev.<HOSTNAME>`.

---

## Install on a fresh server (one command)

Ubuntu/Debian only. DNS for your chosen hostname must already point to the server, and you'll want a wildcard `*.dev.<HOSTNAME>` pointing there too for per-project dev URLs.

```bash
curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/install.sh \
  | sudo bash -s -- remote.example.com
```

The top-level `install.sh` is a thin shim that clones the repo and hands off to [`infra/install.sh`](infra/install.sh). That orchestrator runs each step in [`infra/steps/`](infra/steps/) in order:

1. **`01-host-deps.sh`** — apt base, Node 20, Go, Caddy, claude CLI, LXD via snap, systemd-resolved drop-in for `*.lxd`
2. **`02-code-server.sh`** — code-server `.deb`, config rendered from [`infra/templates/code-server-config.yaml.tmpl`](infra/templates/code-server-config.yaml.tmpl), `code-server@root.service` enabled
3. **`03-app.sh`** — clone repo to `/opt/remote.futrx.dev`, build frontend (`vite build` → `backend/public/`), build backend (`go build` → `backend/remote`), seed OAuth config if `--google-client-id=` / `--google-client-secret=` passed
4. **`04-caddy.sh`** — render [`infra/templates/Caddyfile.tmpl`](infra/templates/Caddyfile.tmpl), validate, atomically replace `/etc/caddy/Caddyfile`, reload
5. **`05-backend-svc.sh`** — render [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl), enable + start, health-check, open UFW

After install:

```bash
claude login   # interactive — authenticates the Claude CLI under /root/.claude
```

then open `https://your-hostname/`.

> ⚠ Unless you pass Google OAuth flags to the installer, the URL is open to anyone on the internet and Claude has full host access.

---

## Updating the server

Two paths depending on what you changed.

### Code-only change (backend Go, frontend Preact)

Just push to `main`. CI ([`.github/workflows/deploy.yml`](.github/workflows/deploy.yml)) takes the **fast path**:

1. `git reset --hard origin/main` on the box
2. `npm install` + `vite build`
3. `go build` → `backend/remote`
4. `systemctl restart remote.futrx.dev`
5. health check on `127.0.0.1:7682`

Typical time: ~25s. Does **not** touch Caddy, host packages, code-server, systemd unit, or LXD.

### Infra change (anything under `infra/`)

Any edit to `infra/templates/*` (Caddyfile, systemd unit, code-server config), `infra/steps/*.sh` (host deps, LXD config, code-server install), or `infra/install.sh` itself, needs the full installer to land on the box. Two ways:

**(a) Trigger the workflow with `installer=true`** — preferred, runs through CI with the same SSH setup:

```bash
gh workflow run deploy --field installer=true
```

or in the GitHub Actions UI: **Deploy → Run workflow → check "installer"**.

**(b) SSH in and run it directly** — useful when iterating:

```bash
ssh root@remote.futrx.dev 'bash /opt/remote.futrx.dev/infra/install.sh remote.futrx.dev --skip-dns-check'
```

Either way, `infra/install.sh` is fully idempotent — re-running it on an up-to-date box is a ~25s no-op.

### Caddyfile edits

- **Permanent**: edit [`infra/templates/Caddyfile.tmpl`](infra/templates/Caddyfile.tmpl), commit, then run the installer (path (a) or (b) above).
- **Experimental** (gets overwritten next time the installer runs): SSH in, edit `/etc/caddy/Caddyfile` directly, `systemctl reload caddy`.

### Adding a host dependency

Edit [`infra/steps/01-host-deps.sh`](infra/steps/01-host-deps.sh) — guard your install with `command -v X` or `dpkg -l X` so it's a no-op when the dep is already there — commit, then trigger the installer.

### Bumping code-server

Bump `CODE_SERVER_VERSION` near the top of [`infra/steps/02-code-server.sh`](infra/steps/02-code-server.sh), commit, then trigger the installer. The step detects a version mismatch and re-installs the `.deb`.

---

## Process supervision (systemd)

Every long-running piece of the stack is a systemd unit, so a crash gets restarted automatically and a host reboot brings everything back without intervention. There's **no separate frontend process** — the Preact SPA is built into `backend/public/` at deploy time and embedded into the Go binary via `//go:embed`, so serving the UI is the backend's job.

| Unit | Source | What it runs | Listens on | Restart | Auto-start on reboot |
|---|---|---|---|---|---|
| `remote.futrx.dev.service` | template at [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl), rendered by `infra/steps/05-backend-svc.sh` | `/opt/remote.futrx.dev/backend/remote` (Go binary, API + WS + embedded SPA) | `127.0.0.1:7682` | `Restart=always`, `RestartSec=2` | yes (`WantedBy=multi-user.target`) |
| `caddy.service` | shipped by the Cloudsmith `caddy` .deb; we only manage `/etc/caddy/Caddyfile` (template at [`infra/templates/Caddyfile.tmpl`](infra/templates/Caddyfile.tmpl)) | Caddy edge: TLS termination, OAuth forward_auth, `*.dev.HOSTNAME` wildcard, on-demand certs | `*:80` and `*:443` | `Restart=on-abnormal` (Caddy's shipped default) | yes (enabled by `infra/steps/04-caddy.sh`) |
| `code-server@root.service` | shipped by the `code-server` .deb as a template unit; we install the .deb in `infra/steps/02-code-server.sh` and render its config from [`infra/templates/code-server-config.yaml.tmpl`](infra/templates/code-server-config.yaml.tmpl) | VS Code in the browser at `code.HOSTNAME` (auth=none; Caddy gates) | `127.0.0.1:8080` | `Restart=always` (code-server's shipped default) | yes |
| `snap.lxd.daemon.service` | shipped by snap | LXD daemon — supervises every `proj-<slug>` container | LXC API socket; bridge `lxdbr0` | snap watchdog | yes; containers come back because `infra` sets `boot.autostart=true` on each one |
| `systemd-resolved.service` | built-in | Reads `/etc/systemd/resolved.conf.d/lxd.conf` (written by `infra/steps/01-host-deps.sh`) so the host can resolve `<container>.lxd` via LXD's bridge dnsmasq | UDP/TCP `53` (loopback) | systemd built-in | yes |

### What this gives you

- **Process crash** → its unit restarts it in ~2s. The other services keep running; clients get one brief failed request and then it's back.
- **Host reboot** → all units come back via `multi-user.target`, LXD containers come back via `boot.autostart=true`, full convergence in ~10–20s.
- **OOM-kill** of any process → same as a crash.
- **Deploy** → CI runs `systemctl restart remote.futrx.dev` to swap the binary cleanly. `KillMode=process` on the backend unit spares its child processes (claude streams already in flight), so an active prompt finishes; only fresh prompts during the restart window get a momentary connection error and reconnect via the SPA's WS auto-handling.

### Where the unit files come from

Two patterns:

1. **We own the unit file** → only `remote.futrx.dev.service`. The installer renders [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) into `/etc/systemd/system/remote.futrx.dev.service`, then `systemctl daemon-reload` + `enable --now`. Edit the template (not the deployed file) to change it, then trigger the full installer.
2. **The package ships its own unit file** → `caddy`, `code-server`, `snap.lxd.daemon`. We just install the package and enable the unit. To change *their* behavior you typically edit the package's config file (which we DO own — `/etc/caddy/Caddyfile`, `/root/.config/code-server/config.yaml`), not the unit. Drop-in overrides go in `/etc/systemd/system/<unit>.d/` and would currently be hand-managed (we have none today).

### Quick inspection

```bash
# What's up right now?
systemctl status remote.futrx.dev caddy code-server@root snap.lxd.daemon

# Live logs
journalctl -u remote.futrx.dev -f

# Did any of them restart recently and why?
systemctl show remote.futrx.dev -p NRestarts -p ActiveEnterTimestamp

# Which ports are bound by what?
ss -tlnp '( sport = :80 or sport = :443 or sport = :7682 or sport = :8080 )'

# Project containers
lxc list
```

If a unit ever shows `inactive (dead)`, look at its `Restart=` directive (`systemctl cat <unit>`) — that's the contract for whether it'll come back on its own.

---

## License

Internal — no external license assigned. Don't redistribute without asking.
