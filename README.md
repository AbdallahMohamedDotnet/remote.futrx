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
| Bake a newer base image | `FORCE_REBUILD_BASE_IMAGE=1 sudo bash /opt/remote.futrx.dev/infra/install.sh <HOSTNAME> --skip-dns-check`<br>or just `cd backend && go run ./cmd/build-base-image -overwrite` | ~120s |

Existing project containers keep their old image version after a rebake — see [`docs/base-image.md`](docs/base-image.md) for converging them.

## Layout

```
backend/
├── cmd/remote/                  the server
├── cmd/build-base-image/        bakes the futrx-remote-dev-base LXD image
└── internal/
    ├── manager/containers/      LXD container lifecycle, auth bundles, image build
    ├── service/claudelogin/     host-side `claude auth login` workflow
    ├── manager/codexauth/       host-side `codex login --device-auth` driver
    ├── integration/lxc/         thin wrapper around the `lxc` CLI
    ├── service/                 chat, project, prompt, auth
    └── transport/               http handlers + ws sockets

frontend/                        Preact SPA, embedded into the Go binary at build time

infra/
├── install.sh                   orchestrator
├── steps/
│   ├── 01-host-deps.sh          apt + Node 20 + Go + Caddy + agent CLIs + LXD
│   ├── 02-code-server.sh        code-server + systemd unit
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
| Host dependency | [`infra/steps/01-host-deps.sh`](infra/steps/01-host-deps.sh) — re-runs converge agent CLIs to their pins |
| Claude/Codex versions | [`agent-cli-versions.env`](backend/internal/manager/containers/agent-cli-versions.env) — shared by the host installer, base image, and container self-healing |
| code-server version | `CODE_SERVER_VERSION=` in [`infra/steps/02-code-server.sh`](infra/steps/02-code-server.sh) |
| Backend systemd unit | [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) |
| Base-image contents (Node, Claude, Codex) | `BaseImageInstallScript` in [`backend/internal/manager/containers/baseimage.go`](backend/internal/manager/containers/baseimage.go) — stale Claude/Codex binaries upgrade in place on next use |

## Services (systemd)

The Preact SPA is embedded in the Go binary at build time, so the backend serves both — no separate frontend process.

| Unit | Source | Listens | Restart |
|---|---|---|---|
| `remote.futrx.dev.service` | [`infra/templates/remote.futrx.dev.service.tmpl`](infra/templates/remote.futrx.dev.service.tmpl) | `127.0.0.1:7682` | `always`, 2s |
| `caddy.service` | Cloudsmith `.deb`; we manage only `/etc/caddy/Caddyfile` | `*:80`, `*:443` | `on-abnormal` |
| `code-server@root.service` | upstream template; config from [`infra/templates/code-server-config.yaml.tmpl`](infra/templates/code-server-config.yaml.tmpl) | `127.0.0.1:8080` | `always` |
| `snap.lxd.daemon.service` | snap | LXD API socket; bridge `lxdbr0` | snap watchdog |

All units `enable`d → come back on host reboot. Project containers come back because the backend sets `boot.autostart=true` on each at launch time (see `containers.Manager.EnsureBootAutostart`). `KillMode=process` on the backend unit keeps in-flight agent subprocesses alive across deploys.

Inspect:

```bash
systemctl status remote.futrx.dev caddy code-server@root snap.lxd.daemon
journalctl -u remote.futrx.dev -f
lxc list
ss -tlnp '( sport = :80 or sport = :443 or sport = :7682 or sport = :8080 )'
```

## Template variables

`render_template` in [`infra/install.sh`](infra/install.sh) is a whitelisted `envsubst` wrapper, so stray `$something` in templates (e.g. Caddy's `{re.host.1}`) is left alone.

| Var | Source | Used in |
|---|---|---|
| `HOSTNAME` | CLI arg | every template |
| `HOSTNAME_RE` | derived (dots escaped) | `Caddyfile.tmpl` |
| `INSTALL_DIR` | `/opt/remote.futrx.dev` | `remote.futrx.dev.service.tmpl` |
| `SERVICE_PORT` | `7682` | `Caddyfile.tmpl`, `remote.futrx.dev.service.tmpl` |
| `CODE_SERVER_PORT` | `8080` | `code-server-config.yaml.tmpl`, `Caddyfile.tmpl` |
| `LXD_BRIDGE_IP` | detected from `lxc network` | `lxd-resolved.conf.tmpl` |
