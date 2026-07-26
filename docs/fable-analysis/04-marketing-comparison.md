# Remote — give your AI agents a real computer

*Marketing one-pager + competitive comparison. Product claims are grounded in the 2026-07-22
demo recording (see [01-video-walkthrough.md](01-video-walkthrough.md)); competitor facts are
current as of July 2026 and cited at the end.*

---

## The pitch

**Every AI coding agent you use today is trapped on your laptop. Remote gives each of your
projects its own full Linux computer in the cloud — and lets Codex, Claude, and Kimi live
there, together, 24/7.**

Remote is a self-hosted browser workspace for coding agents. You install it on any VPS. Each
project you create is a complete, isolated Ubuntu container — its own CPU, RAM, disk, network,
IDE, and browser. You talk to it from a chat tab on your laptop or phone; the agents do the
work on your server, at your server's speed, whether or not your machine is even on.

> "When you give the LLM every permission it needs, every tool, and an entire computer — it
> turns into something out of this world." — from the demo narration

## Why it hits different

**1. A project is a computer, not a folder.**
Other tools sandbox an agent inside a directory on the machine you're typing on. Remote
provisions a full LXC container per project — Ubuntu 24.04, Node, the agent CLIs, and VS Code
(code-server) pre-installed. The agent can `apt install` anything, run any server, break
anything — and it never touches your machine or your other projects. Delete the project,
delete the computer. Completely safe by construction.

**2. Three agents, one chat. No vendor lock-in.**
Codex, Claude, and Kimi are tabs in the same composer. Pick your model (e.g. GPT-5.6 Sol),
thinking level, speed, and mode per prompt — and switch provider mid-conversation if one is
underperforming. Sign in once on the host with your existing subscriptions (ChatGPT, Claude,
Kimi — no API-key billing) and every container inherits the credentials.

**3. Truly parallel, truly unlimited.**
Open as many projects and chats as your server can hold. In the demo, three agent runs build
three different deliverables in two containers *at the same time* — a YouTube-transcript web
app, a marketing site scraped from live community data, and a promotional video pipeline
(agent-installed Playwright, FFmpeg, and fonts).

**4. The Agent Browser: computer use that never sleeps.**
Flip one toggle and a real headed Chrome opens *inside the container*, streamed to you over
encrypted VNC. Log in to a site once, solve the CAPTCHA once — the agent then works inside
your authenticated session around the clock, from the server's IP, at datacenter speed
(2.8 Gbps on fast.com in the demo). It's the computer-use concept without borrowing your
computer.

**5. Ship on a URL, instantly.**
Anything an agent serves gets a public, TLS'd address on your domain —
`https://<project>--<port>.dev.your-host` — gated by project membership. Add a teammate by
email, they sign in with Google, and they get the whole workspace: chats, terminal, files,
uploads, browser.

**6. Your whole dev environment lives in a tab.**
Terminal, file manager with downloads, Git history with checkout, full VS Code, and a live
app preview with an element picker — click any element on your rendered page and its computed
CSS + HTML lands in your next prompt ("make this headline more interesting"). The client cost
of all this? The desktop wrapper idles at **~60 MB** next to WebStorm's 3.18 GB — because the
real computing happens on the server.

**7. Agents that upgrade themselves.**
Skills are files in the project. Ask the agent to "build a skill that knows how to transcribe
videos" and it writes one into `/workspace/.agents/skills/` — the skill picker updates live.
The demo author runs his entire YouTube pipeline this way: motion graphics, video editing,
and autonomous uploads, built as skills over two months of daily use.

**8. Secrets your agent can use but never see.**
Per-project secrets are injected as environment variables on each run — never written into
the container, never synced back, never shown to the model. Vercel-style ergonomics for
agent credentials.

**Also:** voice dictation in any language (the demo mixes Arabic and English mid-sentence),
prompt queueing while agents work, message rewind, chat forking, a PWA that runs from your
phone, and per-project CPU/RAM/disk limits you can change live.

## Who it's for

- **Solo builders** who want five agents building five things overnight without buying a
  bigger laptop.
- **Content creators & operators** automating real workflows (scraping, video, social,
  publishing) that need logins, tools, and long-running processes.
- **Small teams** who want to hand a client or teammate a URL — not a repo, a setup guide,
  and a prayer.
- **Anyone burned by "agent ran `rm -rf` in the wrong terminal"** — isolation isn't a
  setting here; it's the architecture.

---

## How Remote compares

The honest framing: **Claude Code** and the **Codex app** are best-in-class *agent clients*
that run on (and are limited by) your machine and their vendor's models. **OpenClaw** and
**Hermes Agent** are self-hosted *personal-assistant frameworks* built around messaging, not
development. Remote occupies the empty quadrant: **self-hosted, development-first, and
multi-vendor — with hard container isolation per project.**

| | **Remote** | **Claude Code (desktop)** | **Codex app (OpenAI)** | **OpenClaw** | **Hermes Agent** |
|---|---|---|---|---|---|
| Category | Self-hosted agent workspace | Vendor coding agent (CLI + desktop) | Vendor coding agent (in ChatGPT desktop) | Open-source personal assistant | Open-source personal agent framework |
| Runs on | **Your VPS** — laptop can be off | Your machine (+ vendor cloud sessions) | Your machine (+ vendor cloud tasks) | Your machine / self-hosted | Your machine / self-hosted |
| Models / agents | **Codex + Claude + Kimi in one chat**, switchable mid-conversation | Anthropic models | OpenAI models (GPT-5.6) | Model-agnostic (BYOK) | Model-agnostic (BYOK) |
| Isolation | **Full LXC container per project** — separate OS, resources, network | Git-isolated parallel sessions on your OS | Threads/projects on your OS; sandboxed cloud tasks | Runs with broad access on the host it lives on | Runs on the host it lives on |
| Uses your existing subscriptions | **Yes** — ChatGPT, Claude, Kimi sign-in; no API billing | Claude subscription | ChatGPT subscription | No — you supply API keys | No — you supply API keys |
| Dev tooling in the box | **Terminal, file manager, Git UI, full VS Code (code-server), live app preview + element picker** | Integrated terminal, editor, diff viewer, app previews | Inline diffs, PR review, multi-repo projects | None (assistant-first; can run shell commands) | Basic (can write/run code; no IDE) |
| Browser / computer use | **Server-side headed Chrome, 24/7, shared login sessions (VNC)** | In-app browser + computer use on your Mac | In-app browser + computer use (macOS) on your Mac | Web automation skills on your host | Web search/browse tools |
| Publish work to a URL | **Yes — `project--port.dev.<host>` with TLS + membership gating** | Local previews | Local previews / cloud PRs | No | No |
| Team / multi-user | **Yes — per-project members, Google SSO** | Per-seat; enterprise config | Per-seat | Single-operator | Single-operator |
| Phone access | **Yes — PWA, full workspace** | Dispatch sessions from phone | ChatGPT mobile hand-off | iOS/Android companion apps | Via messaging apps (Telegram etc.) |
| Skills / extensibility | Per-provider skill sets, project-scoped; **agent can author its own skills** | Skills, plugins, MCP connectors | 90+ plugins, automations, memory | 100+ community AgentSkills | **Auto-learns skills after complex tasks** |
| Secrets handling | **Vault per project; injected per-run, hidden from the model** | Env/config on your machine | Env/config on your machine | 1Password vault integration | Config on host |
| Resource ceiling | **Your server's — per-project CPU/RAM/disk limits, adjustable live** | Your laptop's RAM/CPU | Your laptop's RAM/CPU | Your host's | Your host's |
| Blast radius if an agent goes rogue | **One disposable container** | Your workstation (session scope) | Your workstation (session scope) | Your workstation + connected accounts | Your host + connected accounts |
| Open / self-hostable | **Yes — your domain, your data** | No (client is free; service is vendor) | No | Yes (open source) | Yes (open source) |

### vs Claude Code

Claude Code is arguably the best single-vendor coding agent, and its 2026 desktop app added
parallel git-isolated sessions, an in-app browser, computer use, and phone dispatch. But it is
one vendor's models running against your machine: parallelism is bounded by your laptop, "computer
use" borrows *your* screen, and there's no notion of handing a running workspace to a teammate
as a URL. **Remote runs Claude Code itself inside each container** — you keep everything
Claude Code is, and gain the server, the isolation, the multi-vendor switch, and the always-on
Agent Browser on top.

### vs the Codex app

Codex (now merged into the ChatGPT desktop app) brings memory, plugins, computer use, and
multi-repo projects — inside OpenAI's walls, on your Mac or Windows machine. Remote runs the
same Codex CLI on your subscription, but in a place where it can `apt install` whatever a task
needs, serve the result on a real domain, and keep working after you close the laptop. When
GPT-5.6 has a bad day, Claude or Kimi is one tab away — in the same conversation.

### vs OpenClaw

OpenClaw went viral as the self-hosted *life* assistant: WhatsApp/Telegram/Slack integration,
100+ community AgentSkills, heartbeat scheduling, local memory. It's a companion, not a
workshop — there's no per-project isolation, no IDE, no git tooling, no preview URLs, and the
agent operates with broad access to the single host it lives on (its security posture has been
a running community concern). Remote is the inverse: hard walls between projects, a full
development stack inside each, and the "always-on personal automation" story delivered through
skills plus a shared authenticated browser — with the blast radius of a throwaway container.

### vs Hermes Agent

Hermes (Nous Research) is the tinkerer's framework — model-agnostic, persistent memory, cron
scheduling, and a genuinely clever closed learning loop where the agent writes its own skills
after completing hard tasks. Remote shares the self-hosted DNA and even the self-authored
skills idea (demonstrated live in the demo), but Hermes is a framework you assemble around one
host and one operator. Remote is a finished multi-user product: provisioned containers,
resource quotas, TLS preview domains, team sharing, secrets, an IDE, and three commercial
agent CLIs signed in and ready.

---

## The one-liner

> **Claude Code and Codex gave agents hands. Remote gives them a home.**
> Self-hosted. Isolated. Multi-agent. Always on.

---

### Competitor sources (July 2026)

- Claude Code desktop: [claude.com/blog — desktop redesign](https://claude.com/blog/claude-code-desktop-redesign),
  [code.claude.com/docs — desktop](https://code.claude.com/docs/en/desktop),
  [9to5Mac — in-app browser](https://9to5mac.com/2026/07/10/anthropic-highlights-claude-codes-in-app-browser-on-the-desktop/),
  [VentureBeat — redesign & Routines](https://venturebeat.com/orchestration/we-tested-anthropics-redesigned-claude-code-desktop-app-and-routines-heres-what-enterprises-should-know)
- Codex app: [openai.com — Introducing the Codex app](https://openai.com/index/introducing-the-codex-app/),
  [Codex changelog](https://developers.openai.com/codex/changelog),
  [Help Net Security — Codex desktop computer use](https://www.helpnetsecurity.com/2026/04/17/openai-codex-desktop-update-macos/),
  [Coursiv — Codex merged with ChatGPT app](https://coursiv.io/blog/codex-merged-with-chatgpt-app)
- OpenClaw: [openclaw.ai](https://openclaw.ai/),
  [DigitalOcean — What is OpenClaw](https://www.digitalocean.com/resources/articles/what-is-openclaw),
  [emergent.sh — OpenClaw guide](https://emergent.sh/learn/what-is-openclaw)
- Hermes Agent: [hermes-agent.org](https://hermes-agent.org/),
  [GitHub — NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent/releases),
  [AI.cc — Hermes Agent 2026 guide](https://www.ai.cc/blogs/hermes-agent-2026-self-improving-open-source-ai-agent-vs-openclaw-guide/)
