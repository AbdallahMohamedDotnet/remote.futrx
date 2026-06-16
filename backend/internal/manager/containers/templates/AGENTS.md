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

## Authenticated browser actions

The platform ships a generic Playwright wrapper at
**`/workspace/scripts/browser.mjs`**. Use it to screenshot, record, or
drive any site the project has authenticated to:

```bash
node /workspace/scripts/browser.mjs screenshot https://app.example.com/dashboard
node /workspace/scripts/browser.mjs record     https://app.example.com/flow --duration 8000
```

Output paths print on stdout; files land in `/workspace/.browser/`. Use
your `Read` tool on PNGs; videos are `.webm`.

The script reads **`/workspace/.agents/browser-auth.json`** to know which
cookie to attach for each host. To add a new site:

1. Add one entry to the JSON yourself (the file is empty by default):
   ```json
   {
     "app.example.com": {
       "cookies": [
         { "name": "<cookie-name>", "domain": "<cookie-domain>",
           "secret": "<ENV_VAR>", "path": "/",
           "httpOnly": true, "secure": true, "sameSite": "None" }
       ]
     }
   }
   ```
   `secret` is the **name** of an env-var (e.g. `LINEAR_SESSION`) — never
   put the cookie value into this file.
2. Ask the user to paste the cookie value into **Containers → Secrets**
   under the env-var name you chose. Tell them which cookie to copy:
   *DevTools → Application → Cookies → `<cookie-domain>` →
   `<cookie-name>` → copy the Value column.*

**Don't type passwords or complete "Sign in with Google / Apple" flows
headlessly** — automated browsers are detected and refused. For sites that
need a real login, use the **Agent Browser** below: the user logs in by
hand once, then you drive that same authenticated session. The cookie path
above stays available for sites where the user can copy a session cookie.

**If a script suddenly returns a logged-out page**, the cookie has
rotated. Tell the user to re-paste a fresh value — don't silently retry.

### Agent Browser — act in a session the user logged into

For sites with no usable API and no copyable cookie (most social logins),
the platform can run a **real, visible browser inside this container** that
the user logs into by hand, after which you drive the *same* session:

1. Ask the user to open **Browser → Agent browser** in the chat UI and log
   into the target site there (they see a live view and handle the password
   and any 2FA). Their login persists across container restarts.
2. Drive that live, already-authenticated session with the **`connect`**
   subcommand — it attaches over CDP and shares the user's cookies, so you
   never handle credentials:

   ```bash
   # report what's open in the live session
   node /workspace/scripts/browser.mjs connect
   # run a recipe against the logged-in session
   node /workspace/scripts/browser.mjs connect /workspace/.browser/recipes/<scenario>.mjs
   ```

   The recipe shape is identical to `run` (default async function
   `(page, context)`), but it acts in the live profile — no
   `browser-auth.json` cookies are attached.

**Write policy:** reading (timelines, messages, search) is fine on your
own. Before any **public or irreversible write** — posting, replying,
DMing, following, purchasing, changing settings — **say what you're about
to do and get the user's confirmation first.** They can also watch and stop
you through the live view.

**Egress note:** this browser exits via the datacenter IP, so strict
providers (Google, X) may show extra "verify it's you" challenges at login;
the user clears those in the live view. Respect each site's terms of service.

### Recording agent-driven flows (clicking, filling, multi-step)

When the user asks you to record a click sequence, fill out a form, or
demonstrate a multi-step interaction, write a recipe and let the generic
script drive it:

```js
// /workspace/.browser/recipes/<scenario>.mjs
export default async function (page, context) {
  await page.goto('https://app.example.com/dashboard');
  await page.waitForLoadState('networkidle');
  await page.click('text=Analytics');
  await page.waitForTimeout(1500);
  await page.click('text=Reports');
  await page.waitForTimeout(1500);
  // optional: return a value, it'll print as JSON on stdout
  // return { title: await page.title() };
};
```

Then invoke it via the **`run`** subcommand:

```bash
node /workspace/scripts/browser.mjs run /workspace/.browser/recipes/dashboard-tour.mjs --record
```

The generic script handles the cookie setup, viewport, video recording,
cleanup, and path-printing. **Every cookie from every entry in
`browser-auth.json` whose secret is set gets attached up-front**, so the
recipe can navigate to any registered site without you having to specify
which.

Flags:
- `--record`        record a `.webm` of the run (omit for headless action only)
- `--out <path>`    where to write the video (default: `/workspace/.browser/<auto>.webm`)
- `--timeout <ms>`  abort the recipe if it runs longer (default 300000 = 5 min)

Recipes are throwaway — put them in `/workspace/.browser/recipes/` and
delete them when you're done with the scenario. **Don't reach for a
recipe for a single screenshot** — that's what the `screenshot` and
`record` subcommands are for.

### Playwright install

If Playwright isn't installed yet, `scripts/browser.mjs` will print the
one-time install command — run it, then retry.

## Project skills

Project-scoped skills have one source of truth:

- `/workspace/.agents/skills/<name>/SKILL.md`

The host provisions that directory at launch and before each prompt. It also
keeps compatibility symlinks so Claude and legacy Codex paths resolve to the
same files:

- `/workspace/.claude/skills -> ../.agents/skills`
- `/workspace/.codex/skills -> ../.agents/skills`

When suggesting that the user create a new project skill, use the
`/workspace/.agents/skills/` location. Never duplicate the same skill into
`.claude/` or `.codex/`.
