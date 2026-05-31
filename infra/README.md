# infra/ — server provisioning for remote.futrx.dev

Everything needed to bring a fresh Ubuntu/Debian box from zero to a running
`remote.futrx.dev` install. Used by both the **first-time installer** and the
**CI deploy pipeline**.

## Layout

```
infra/
├── README.md                       this file
├── install.sh                      orchestrator — runs each step in order
├── steps/
│   ├── 01-host-deps.sh             apt + Node 20 + Go + Caddy + claude CLI + LXD
│   ├── 02-code-server.sh           code-server install + systemd + config render
│   ├── 03-app.sh                   repo clone/update + build + auth-secret seed
│   ├── 04-caddy.sh                 Caddyfile render from template + reload
│   ├── 05-backend-svc.sh           backend systemd unit + start + health + firewall
│   └── 06-base-image.sh            bake futrx-remote-dev-base LXD image (idempotent)
└── templates/
    ├── Caddyfile.tmpl              edge config: main host, code subdomain, dev wildcard
    ├── remote.futrx.dev.service.tmpl   backend systemd unit
    ├── code-server-config.yaml.tmpl    code-server config (auth=none; Caddy gates)
    └── lxd-resolved.conf.tmpl      systemd-resolved drop-in for *.lxd
```

Each step is **idempotent**: re-running it on an already-installed box is safe
and fast (commands skip themselves when their work is already done). That makes
`install.sh` the canonical thing to run on every CI deploy, not just on first
install — see `.github/workflows/deploy.yml`.

## Usage

**Fresh install** (via curl-pipe-to-bash, see the README at repo root):

```bash
curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/install.sh \
  | sudo bash -s -- remote.example.com
```

The top-level `install.sh` at the repo root is a small shim that clones the
repo if needed and execs `infra/install.sh`.

**Re-run from a clone** (after editing infra):

```bash
sudo bash infra/install.sh remote.example.com --skip-dns-check
```

**Per-step (debugging only)**: each `steps/NN-*.sh` can be sourced in isolation
once you've set the env vars `infra/install.sh` would normally set
(`HOSTNAME`, `INSTALL_DIR`, `SERVICE_PORT`, etc.). Read the top of each step
to see what it expects.

## Templates

Templates use `${VARNAME}` placeholders that `envsubst` fills in. The wrapping
helper `render_template` in `infra/install.sh` whitelists only the variables
it knows about, so stray `$something` in the template (e.g. Caddy's
`{re.host.1}`) isn't mangled.

Variables exposed to templates:

| Var | Source | Used in |
|---|---|---|
| `HOSTNAME` | CLI arg | every template |
| `HOSTNAME_RE` | derived (dots escaped) | `Caddyfile.tmpl` |
| `INSTALL_DIR` | `/opt/remote.futrx.dev` | `remote.futrx.dev.service.tmpl` |
| `SERVICE_PORT` | `7682` | `Caddyfile.tmpl`, `remote.futrx.dev.service.tmpl` |
| `CODE_SERVER_PORT` | `8080` | `code-server-config.yaml.tmpl`, `Caddyfile.tmpl` |
| `LXD_BRIDGE_IP` | detected from `lxc network` | `lxd-resolved.conf.tmpl` |
