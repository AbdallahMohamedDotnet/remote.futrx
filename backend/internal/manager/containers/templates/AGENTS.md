# Sandbox: remote.futrx.dev

You're running inside an unprivileged LXC container, one per project,
spawned by [remote.futrx.dev](https://remote.futrx.dev). Other
projects can't see this one - fresh `apt` installs, crashed
processes, deleted files only affect you.

## Filesystem

- `/workspace` - your project files. Persistent, survives container
  restarts and reprovisions.
- `/root/.claude/CLAUDE.md` AND `/root/.codex/AGENTS.md` - this file
  (identical content, two paths so both Claude and Codex pick it up).
  The host re-pushes both whenever the template changes; don't edit
  them expecting changes to stick.
- Everything else: ephemeral.

## Capabilities

- Root in the container (uid 0). Unprivileged means container-root
  maps to a low-privilege host user - no host escape.
- `apt-get install` whatever you need.
- Network is fully open.
- Background processes persist between prompts; they die on container
  stop/reboot.

## Pre-installed tools

`git`, `gh`, `openssh-client`, `jq`, `build-essential`,
`python3` + `pip`, `node 20` + `npm`, `claude`, `codex`. Anything else:
`apt-get install` or `npm i -g` freely.

**Persistence rule.** `/workspace/**` is a host bind-mount and survives
container delete. Everything else (`/usr/local/`, `/root/`, packages you
apt-install) is gone if the container is recreated. If you install a
tool the project needs again later, append the install line to
`/workspace/setup.sh` so a fresh container can rebootstrap with
`bash /workspace/setup.sh`.

## Secrets

Tokens are project-scoped. Ones the user has configured are exported as
env vars *and* mirrored to `/workspace/.env`. CLIs that read env (`gh`,
`wrangler`, `hcloud`, `aws`, …) pick them up automatically — no
`source`, no `--token` flag, nothing.

Discover what's currently set:

```bash
env | cut -d= -f1 | grep -E '_(TOKEN|KEY|SECRET|PASSWORD)$|^(GITHUB|CLOUDFLARE|HCLOUD|OPENAI|ANTHROPIC|AWS|GOOGLE)_' | sort
# or, for the human-readable list:
cat /workspace/.env 2>/dev/null
```

If you need a credential that isn't set, ask the user to add it via **this
project's Containers → Secrets** in the web UI. Use the canonical
env-var name the upstream CLI expects — never invent your own. Common
ones:

| Provider | Env var | Generate at |
|---|---|---|
| GitHub (git clone / push) | `GITHUB_SSH_KEY` (paste the **private** key — full PEM, including BEGIN/END lines) | https://github.com/settings/keys → "New SSH key" (paste the matching public key) |
| Cloudflare | `CLOUDFLARE_API_TOKEN` | https://dash.cloudflare.com/profile/api-tokens |
| Hetzner Cloud | `HCLOUD_TOKEN` | console.hetzner.cloud → Security → API Tokens |
| OpenAI | `OPENAI_API_KEY` | https://platform.openai.com/api-keys |
| Anthropic | `ANTHROPIC_API_KEY` | https://console.anthropic.com/settings/keys |
| AWS | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` | IAM → Users → Security credentials |
| Google Cloud | `GOOGLE_APPLICATION_CREDENTIALS_JSON` (service-account JSON, raw) | IAM → Service Accounts → Keys |
| npm registry | `NPM_TOKEN` | https://www.npmjs.com/settings/~/tokens |

New values are live on your *next* shell. Already-running processes
(dev servers you started earlier, etc.) keep their old environ — kill
and restart them to pick up new credentials.

## Dev servers — there is no localhost, there is a public URL

Whenever the user asks for a dev server, **the URL they reach it at
is**:

```
https://<this-project-slug>--<port>.dev.remote.futrx.dev
```

Replace `<this-project-slug>` with this project's slug and `<port>`
with whatever you bound. Pretend `localhost:<port>` doesn't exist
for any user-facing purpose - they're not on this box, they're on
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

### Multi-line secrets (SSH keys, JSON service accounts, PEM certs)

The Secrets value box accepts newlines — paste the whole PEM block, the
whole JSON service account, the whole PKCS#1 blob. The value reaches
this container as a single env var with the newlines intact, and lands
in `/workspace/.env` with the newlines encoded as `\n` escape sequences
(any dotenv library decodes them; raw `cat .env` shows the escapes).

If you need the secret as a file (most ssh / gcloud / certbot use
cases), write it yourself — don't try to source `.env` for those, the
escape encoding will burn you:

```bash
# SSH private key
mkdir -p /root/.ssh && chmod 700 /root/.ssh
printf '%s\n' "$GITHUB_SSH_KEY" > /root/.ssh/id_ed25519
chmod 600 /root/.ssh/id_ed25519
ssh-keyscan github.com >> /root/.ssh/known_hosts 2>/dev/null
git clone git@github.com:org/repo.git    # works

# GCP service account
printf '%s' "$GOOGLE_APPLICATION_CREDENTIALS_JSON" > /root/gcp-key.json
export GOOGLE_APPLICATION_CREDENTIALS=/root/gcp-key.json
```

`printf '%s' "$VAR"` preserves the in-memory value byte-for-byte. Avoid
`echo` for binary-ish content — some shells interpret backslash escapes.

When you need a secret that isn't there yet, tell the user the exact
canonical name (e.g. `GITHUB_SSH_KEY`, `GOOGLE_APPLICATION_CREDENTIALS_JSON`)
and that they should paste the value — including newlines — into the
project's **Containers → Secrets** UI.
