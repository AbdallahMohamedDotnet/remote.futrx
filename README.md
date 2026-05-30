# remote.futrx.dev

A self-hosted Claude Code chat UI with a Go backend. Each "project" in the sidebar is its own unprivileged LXC container; dev servers inside any project are reachable at `<slug>--<port>.dev.<HOSTNAME>`.

---

## Install on a fresh server

Ubuntu/Debian only. DNS for `<HOSTNAME>` and wildcard `*.dev.<HOSTNAME>` must already point to this server.

```bash
curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/infra/install.sh \
  | sudo bash -s -- remote.example.com
```

[`infra/install.sh`](infra/install.sh) self-bootstraps when run from a pipe: installs git, clones the repo to `/opt/remote.futrx.dev`, then re-execs from there and runs each step in [`infra/steps/`](infra/steps/) in order (host deps + LXD → code-server → repo build → Caddyfile → backend systemd). See [`infra/README.md`](infra/README.md) for per-step detail.

After install:

```bash
claude login           # interactive — authenticates the Claude CLI under /root/.claude
```

then open `https://<HOSTNAME>`. Pass `--google-client-id=` / `--google-client-secret=` to the installer to enable Google OAuth; otherwise the URL is open to anyone on the internet.

---

## Updating the server

**Code change** (Go / Preact) → push to `main`. CI fast path: git pull + build + restart backend, ~25s. Doesn't touch Caddy, host deps, or code-server.

**Infra change** (anything under `infra/`) → push, then trigger the full installer:

```bash
gh workflow run deploy --field installer=true
# or: ssh root@<HOSTNAME> 'bash /opt/remote.futrx.dev/infra/install.sh <HOSTNAME> --skip-dns-check'
```

| Editing | Where |
|---|---|
| Caddyfile (permanent) | [`infra/templates/Caddyfile.tmpl`](infra/templates/Caddyfile.tmpl) — then trigger installer |
| Caddyfile (experimental) | `/etc/caddy/Caddyfile` on the box + `systemctl reload caddy` — overwritten next installer run |
| Add a host dep | [`infra/steps/01-host-deps.sh`](infra/steps/01-host-deps.sh) — guard so it's a no-op when present |
| Bump code-server | `CODE_SERVER_VERSION=` in [`infra/steps/02-code-server.sh`](infra/steps/02-code-server.sh) |
| Backend systemd unit | [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) |

`infra/install.sh` is idempotent — re-running on an up-to-date box is a ~25s no-op.

---

## Process supervision (systemd)

The Preact SPA is embedded into the Go binary at build time, so the backend serves both — there's no separate frontend process.

| Unit | Source | Listens | Restart |
|---|---|---|---|
| `remote.futrx.dev.service` | ours; rendered from [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) | `127.0.0.1:7682` | `always`, 2s |
| `caddy.service` | Cloudsmith `caddy` .deb; we only manage [`/etc/caddy/Caddyfile`](infra/templates/Caddyfile.tmpl) | `*:80`, `*:443` | `on-abnormal` |
| `code-server@root.service` | `code-server` .deb template unit; config from [`infra/templates/code-server-config.yaml.tmpl`](infra/templates/code-server-config.yaml.tmpl) | `127.0.0.1:8080` | `always` |
| `snap.lxd.daemon.service` | snap | LXC API socket; bridge `lxdbr0` | snap watchdog |

All units `enable`d → come back on host reboot. Project containers come back because `infra` sets `boot.autostart=true` on each. `KillMode=process` on the backend unit keeps in-flight `claude` subprocesses alive across deploys.

```bash
# Inspect
systemctl status remote.futrx.dev caddy code-server@root snap.lxd.daemon
journalctl -u remote.futrx.dev -f
lxc list
ss -tlnp '( sport = :80 or sport = :443 or sport = :7682 or sport = :8080 )'
```
