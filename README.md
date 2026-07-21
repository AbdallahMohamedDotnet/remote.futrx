# remote.futrx.dev

Self-hosted Claude Code / Codex chat UI. Go server (embeds the Preact SPA) + one unprivileged LXD container per "project". Dev servers inside any project are reachable at `<slug>--<port>.dev.<HOSTNAME>`.

## Install (fresh Ubuntu/Debian)

Prereq: DNS for `<HOSTNAME>` **and** wildcard `*.dev.<HOSTNAME>` already point to this box.

```bash
curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/infra/install.sh \
  | sudo bash -s -- remote.example.com
```

The installer self-bootstraps when run from a pipe: clones the repo to `/opt/remote.futrx.dev`, then runs every step in [`infra/steps/`](infra/steps/) in order. Each step is idempotent — re-running on a configured box is a ~25s no-op.

After it finishes:

```bash
claude login     # writes /root/.claude*, inherited by Claude project runs
# Optional: open Settings and start Codex ChatGPT login for subscription limits
```

Open `https://<HOSTNAME>`. Pass `--google-client-id=` / `--google-client-secret=` to the installer for Google OAuth; otherwise the site is open to anyone who can reach it.

## Update

| Change | Action | Wall-clock |
|---|---|---|
| Go / Preact source | push to `main` — [`deploy`](.github/workflows/deploy.yml) workflow runs git pull + build + restart | ~25s |
| Anything in `infra/` | push, then `gh workflow run installer` | ~30–90s |
| Host Node / Go version | bump the pin in [`infra/versions.env`](infra/versions.env), push, then `gh workflow run installer` — step 01 converges the box to the new pins | ~60s |
| Claude / Codex version | bump the pin in [`agent-cli-versions.env`](backend/internal/agent/provisioning/agent-cli-versions.env), push — host converges on next installer run, containers self-heal on next prompt | ~60s |
| Bake a newer base image | `FORCE_REBUILD_BASE_IMAGE=1 sudo bash /opt/remote.futrx.dev/infra/install.sh <HOSTNAME> --skip-dns-check`<br>or just `cd backend && go run ./cmd/build-base-image -overwrite` | ~120s |
| Upgrade **all** workspaces to a new base image | `sudo bash /opt/remote.futrx.dev/infra/upgrade-workspaces.sh` — rebakes, then recycles every idle container (busy ones skipped; `--dry-run` to preview) | ~2–3min |

Existing project containers keep their old image version after a rebake — see [`docs/base-image.md`](docs/base-image.md) for converging them.

## Layout

```
backend/
├── cmd/remote/                  the server
├── cmd/build-base-image/        bakes the futrx-remote-dev-base LXD image
└── internal/
    ├── agent/                   registered Claude, Codex, and Kimi adapters + auth callers
    ├── integration/containers/  provider-neutral LXD mechanics and image build
    ├── integration/lxc/         thin wrapper around the `lxc` CLI
    ├── service/agent/auth/      shared host-side CLI auth lifecycle service
    ├── service/auth/            user OAuth, sessions, and access policy
    ├── service/                 chat, project, prompt, and supporting services
    └── transport/               HTTP handlers + WebSocket sockets

frontend/                        Preact SPA, embedded into the Go binary at build time

infra/
├── install.sh                   orchestrator
├── update.sh                    re-run installer with the stored hostname (idempotent update)
├── upgrade-workspaces.sh        rebake base image + recycle idle project containers onto it
├── versions.env                 host toolchain pins (Node, Go) — step 01 converges to these
├── steps/
│   ├── 01-host-deps.sh          apt + Node 20 + Go + Caddy + agent CLIs + LXD
│   ├── 03-app.sh                clone/update repo + build + auth-secret seed
│   ├── 04-caddy.sh              Caddyfile render + reload
│   ├── 05-backend-svc.sh        backend systemd unit + start + health + firewall
│   └── 06-base-image.sh         bake futrx-remote-dev-base via cmd/build-base-image
└── templates/                   envsubst-rendered: Caddyfile, systemd units, configs

docs/                            base-image.md, frontend-backend-api.md
```

## Edit map

| Editing | File |
|---|---|
| Caddyfile (permanent) | [`infra/templates/Caddyfile.tmpl`](infra/templates/Caddyfile.tmpl) |
| Caddyfile (experimental) | `/etc/caddy/Caddyfile` + `systemctl reload caddy` — overwritten on next installer run |
| Host dependency | [`infra/steps/01-host-deps.sh`](infra/steps/01-host-deps.sh) — re-runs converge Node, Go, and agent CLIs to the manifest pins |
| Host Node / Go pins | [`infra/versions.env`](infra/versions.env) — shell-owned; bump + re-run installer (or `infra/update.sh`) to upgrade every box |
| Claude/Codex pins | [`agent-cli-versions.env`](backend/internal/agent/provisioning/agent-cli-versions.env) — embedded in the backend; shared by the host installer, base image, and container self-healing |
| code-server version | `CODE_SERVER_VERSION=` in [`code-server-up.sh`](backend/internal/integration/containers/templates/code-server-up.sh) — per-container IDE; also baked into the base image |
| Backend systemd unit | [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) |
| Base-image contents (Node + agent CLIs) | generated by [`backend/internal/integration/containers/baseimage.go`](backend/internal/integration/containers/baseimage.go) from the registration catalog in [`backend/internal/service/agent_catalog.go`](backend/internal/service/agent_catalog.go) |
| Default container resource limits | `profileConfig` in [`resources/manager.go`](backend/internal/integration/containers/resources/manager.go) — backend-managed `futrx-workspace` LXD profile, converged on every launch; per-project override via `lxc config set <container> limits.memory ...` |

## Services (systemd)

The Preact SPA is embedded in the Go binary at build time, so the backend serves both — no separate frontend process.

| Unit | Source | Listens | Restart |
|---|---|---|---|
| `remote.futrx.dev.service` | [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) | `127.0.0.1:7682` | `always`, 2s |
| `caddy.service` | Cloudsmith `.deb`; we manage only `/etc/caddy/Caddyfile` | `*:80`, `*:443` | `on-abnormal` |
| `snap.lxd.daemon.service` | snap | LXD API socket; bridge `lxdbr0` | snap watchdog |

code-server is **not** a host service — each project container runs its own on-demand, idle-stopped instance at `<slug>.lxd:8842` (see [`code-server-up.sh`](backend/internal/integration/containers/templates/code-server-up.sh)).

All units `enable`d → come back on host reboot. Project containers come back because the backend sets `boot.autostart=true` on each at launch time (see `containers.Manager.EnsureBootAutostart`). `KillMode=process` on the backend unit keeps in-flight agent subprocesses alive across deploys.

Inspect:

```bash
systemctl status remote.futrx.dev caddy snap.lxd.daemon
journalctl -u remote.futrx.dev -f
lxc list
ss -tlnp '( sport = :80 or sport = :443 or sport = :7682 )'
```

## Template variables

`render_template` in [`infra/install.sh`](infra/install.sh) is a whitelisted `envsubst` wrapper, so stray `$something` in templates (e.g. Caddy's `{re.host.1}`) is left alone.

| Var | Source | Used in |
|---|---|---|
| `HOSTNAME` | CLI arg | every template |
| `HOSTNAME_RE` | derived (dots escaped) | `Caddyfile.tmpl` |
| `INSTALL_DIR` | `/opt/remote.futrx.dev` | `remote.futrx.dev.service.tmpl` |
| `SERVICE_PORT` | `7682` | `Caddyfile.tmpl`, `remote.futrx.dev.service.tmpl` |
| `LXD_BRIDGE_IP` | detected from `lxc network` | `lxd-resolved.conf.tmpl` |
