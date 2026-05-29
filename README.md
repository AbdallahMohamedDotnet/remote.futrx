# remote.futrx.dev

A self-hosted, mobile-first **Claude Code chat UI** with a Go backend that drives the official `claude` CLI in streaming mode.

Designed to feel like Claude Code Desktop, but accessible from any browser. Built for a single user on a personal box behind cookie auth.

---

## What it does

- One sidebar of persistent **claude chats**, each backed by `claude --resume <session-id>` so context is preserved.
- Token-level streaming UI rendered as proper HTML — markdown, tables, syntax-highlighted code blocks, inline tool-call widgets:
  - **Read** → file pill, expandable content
  - **Edit / MultiEdit** → real unified diff
  - **Write** → file + content preview
  - **Bash** → command + collapsible output
  - **Glob / Grep** → pattern + path + results
  - **AskUserQuestion** → paginated wizard (one question per page, Next disabled until answered)
- **Chatbox** with drag-and-drop, paste-image-from-clipboard, and an attachments grid above the input. Uploaded files land in the chat's working directory; their paths are appended to the prompt on send.
- **Per-chat model selector** (Opus / Sonnet / Haiku) and live token + cost readout in the header.
- A separate tmux-PTY WebSocket endpoint (`/ws`) kept around for SSH-style raw shell access — unused by the SPA but available if needed.

---

## Projects: per-project sandboxes

Each project in the sidebar is its own **unprivileged LXC container** on the host. `claude` runs *inside* the container, not on the host. Chats under a project only see that container's filesystem; the host stays clean.

- `/workspace` is bind-mounted from the host (`/var/lib/remote/projects/<slug>/workspace`) — this is the persistent project dir. Everything else inside the container is ephemeral and wiped on reprovision.
- Public dev URLs: `https://<slug>--<port>.dev.<HOSTNAME>` proxies to `proj-<slug>.lxd:<port>` via a wildcard Caddy block with on-demand TLS. Same Google login as the main UI gates access.
- The `claude` CLI is auto-installed inside the container on first prompt (Node 20 + `@anthropic-ai/claude-code` via npm). Anthropic auth (`~/.claude.json` + `.credentials.json`) is seeded by the host into each container's `/root/` once at provision; the container then mutates its own copy.
- A `CLAUDE.md` briefing is also pushed to `/root/.claude/CLAUDE.md` so the in-container agent knows the shape of its sandbox (filesystem, dev URL pattern, etc.). Template lives at [`backend/internal/projects/templates/CLAUDE.md`](backend/internal/projects/templates/CLAUDE.md); edits re-propagate to every container on its next prompt via a hash-gated push.

### What's isolated vs. shared

`apt install <pkg>` inside a project's container **only affects that container** — its own `/usr/`, `/var/lib/dpkg/`, package versions, configs. Other projects don't see it, the host doesn't see it. Two projects can install conflicting versions of the same package without colliding. Deleting the project deletes the container, so cleanup is total.

What containers **do** share with each other and the host:

| Resource | Shared? | Notes |
|---|---|---|
| Disk pool | yes | one filesystem; 100 projects each running `apt install texlive-full` fills it for everyone |
| Kernel | yes | one Linux kernel; packages needing custom kernel modules won't work |
| Network egress | yes | one uplink |
| RAM / CPU | yes | no per-container quotas today — be a good neighbor |
| IPv4 on the LXD bridge | each container gets its own | host can resolve `<container-name>.lxd` via the bridge's dnsmasq |

So normal "install runtime + deps + build something" work is cleanly scoped per project. The only ways one project can affect another are blunt: disk full, RAM exhausted, CPU pegged.

---

## Architecture

```
Browser
  │
  │  HTTP + WebSocket (cookie auth at Caddy edge)
  ▼
Caddy (edge-caddy container)  ──── magic-link → cookie auth → reverse_proxy
  │
  ▼
Go binary (/opt/remote.futrx.dev/backend/remote)
  │
  ├─ /api/sessions, /api/sessions/{name}/*    tmux session control + upload
  ├─ /api/chats, /api/chats/{id}/*            chat metadata (JSON-file persistence)
  ├─ /ws  ?session=<name>                     tmux PTY bridge (xterm.js)
  ├─ /ws/chat/{id}                            claude stream-json normalizer
  └─ /                                        embedded SPA bundle
  │
  ▼
claude CLI (one process per prompt)
  │
  └─ stream-json → parsed → normalized ChatEvents → appended to events.jsonl + streamed to WS client
```

### Stream-json normalization

`claude -p --output-format stream-json --include-partial-messages --verbose --dangerously-skip-permissions [--model <m>] [--resume <id>]`

Backend parses the JSON event lines, captures the `session_id` once on init, and normalizes into:

```ts
type ChatEvent =
  | { type: "user"; text }
  | { type: "assistant_text"; text }       // streamed from content_block_delta
  | { type: "tool_use_start"; id; name; input }
  | { type: "tool_use_end"; id; output; isError }
  | { type: "session"; claudeSessionId }
  | { type: "complete"; usage }
  | { type: "error"; message }
```

Each event is **appended to `data/chats/{id}/events.jsonl`** and broadcast over the WS. The JSONL file is the replay source on page reload.

---

## Layout

```
.
├── install.sh                  one-shot installer for fresh Ubuntu/Debian
├── README.md
│
├── backend/                    Go service
│   ├── cmd/remote/main.go      config, wiring, server start
│   ├── internal/
│   │   ├── auth/               Google OAuth, cookies, admin middleware
│   │   ├── chat/               chat storage, HTTP handlers, titles
│   │   ├── claude/             Claude stream runner + CLI login bridge
│   │   ├── config/             env vars and defaults
│   │   ├── httpserver/         routes, responses, websocket upgrader
│   │   ├── tmux/               tmux commands + PTY WebSocket bridge
│   │   └── upload/             shared multipart uploads
│   ├── static.go               embedded SPA filesystem
│   ├── go.mod, go.sum
│   ├── public/                 Built SPA bundle — gitignored, written by vite
│   └── remote                  Built binary — gitignored
│
├── frontend/                   Preact + Vite + Tailwind SPA
│   ├── package.json, vite.config.ts, tailwind.config.ts, tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.tsx, App.tsx, types.ts, index.css
│       ├── lib/{api, useChat, usePoll, format}.ts
│       └── components/
│           ├── icons.tsx, ChatSidebar.tsx
│           └── Chat/
│               ├── ChatView.tsx, ChatHeader.tsx, ChatInput.tsx
│               ├── Message.tsx, Markdown.tsx, StreamingText.tsx
│               ├── ToolCall.tsx, AskUserQuestion.tsx
│               └── messageBlocks.ts
│
└── data/chats/{id}/            Runtime state — gitignored
    ├── meta.json               title, claudeSessionId, cwd, model, timestamps
    └── events.jsonl            append-only chat event stream
```

---

## Install on a fresh server (one command)

Ubuntu/Debian only. DNS for your chosen hostname must already point to the server.

```bash
curl -fsSL https://raw.githubusercontent.com/Kings-Of-The-Web/remote.futrx.dev/main/install.sh | sudo bash -s -- remote.example.com
```

The installer:

1. Installs deps (`node 20`, `go`, `tmux`, `caddy`, `@anthropic-ai/claude-code`)
2. Clones the repo to `/opt/remote.futrx.dev`
3. Builds frontend + backend
4. Writes a Caddyfile that reverse-proxies your hostname → `127.0.0.1:7682` with auto-TLS via Let's Encrypt
5. Installs a systemd unit and starts the service
6. Opens 80 + 443 in UFW if active

After install:
```bash
claude login   # interactive — authenticates the Claude CLI under /root/.claude
```
then open `https://your-hostname/`.

Re-running the installer pulls latest, rebuilds, and restarts. Idempotent.

> ⚠ Unless you pass Google OAuth flags to the installer, the URL is open to anyone on the internet, and Claude has full host access.

## Build manually

```bash
cd frontend && npm install && npm run build   # → ../backend/public/{index.html, assets/*}
cd ../backend && go build -trimpath -ldflags="-s -w" -o remote ./cmd/remote
```

Production binary is ~6 MB, static (CGO_ENABLED=0). It lives at `backend/remote`.

Frontend bundle is ~290 KB JS / 90 KB gzipped — Preact + react-markdown + remark-gfm + highlight.js + diff + xterm.

## Run

```bash
HOST=127.0.0.1 PORT=7682 DATA_DIR=/opt/remote.futrx.dev/data backend/remote
```

Reachable only from the Docker `edge` bridge (172.18.0.0/16). Caddy stub container (`/srv/terminal/docker-compose.yml`) carries the public TLS hostname + cookie-auth labels.

### Environment

| Var | Default | What it does |
|---|---|---|
| `HOST` | `172.18.0.1` | Bind interface |
| `PORT` | `7682` | Bind port |
| `DATA_DIR` | `/opt/remote.futrx.dev/data` | Chat metadata + events |
| `HOME` | `/root` | Default cwd for new chats |

### Dependencies on the host

- `claude` (Claude Code CLI) on `$PATH`
- `tmux` ≥ 3.0 (for the unused-but-running PTY bridge)
- Go 1.22+
- Node 18+ (build only)

### Systemd

```
/etc/systemd/system/remote.futrx.dev.service
   ExecStart=/opt/remote.futrx.dev/backend/remote
   WorkingDirectory=/opt/remote.futrx.dev
   KillMode=process    # tmux server (inherited cgroup) survives restarts
   Restart=always
```

`systemctl restart remote.futrx.dev` does not kill running claude streams that have already started — but a fresh `claude -p` won't be spawned during the restart window. Active chats reconnect via WS auto-handling.

---

## Iteration

```bash
# Frontend: HMR dev server, proxies API/WS to Go on :7682
cd frontend && npm run dev

# Backend: rebuild + restart
cd backend && go build -trimpath -ldflags="-s -w" -o remote ./cmd/remote \
  && systemctl restart remote.futrx.dev
```

---

## License

Internal — no external license assigned. Don't redistribute without asking.
