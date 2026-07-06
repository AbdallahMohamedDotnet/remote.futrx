# Agent Browser: Roadmap — Where We Are, Where We're Going

> Companion docs: [agent-browser.md](agent-browser.md) (how the current system
> works) and [agent-browser-phases.md](agent-browser-phases.md) (the phased
> implementation plan for everything below).

## The capability we are building

**Agents get a real browser on the remote Linux server that they can *see*
like a human and *drive* like a human — logged into the user's accounts —
while consuming near-zero resources when idle and a bounded, predictable
amount when active.**

Concretely, that means an agent should be able to take a task like *"check my
X notifications and reply to the important ones"* or *"does the dashboard
render correctly after my change?"* and:

1. Start the browser itself (no human gesture required),
2. Perceive the page **visually** when the task is visual, not just through
   the DOM/accessibility tree,
3. Interact with input that is indistinguishable from a human's (trusted
   events; OS-level input as a fallback),
4. Reuse the user's real logins (which the user performed by hand, once),
5. Leave nothing running when the work is done.

## Where we are today

The current implementation (see [agent-browser.md](agent-browser.md)) already
gets the hardest part right: **one shared headed Chromium per project
container, attached two ways** — a noVNC web view the user watches and logs
in through, and a loopback CDP port the agent drives via `@playwright/mcp`.
The agent inherits the user's cookies without ever touching credentials. The
Chrome profile persists under `/workspace`, so logins survive restarts.

The current implementation also replaces the
WebSocket lifecycle channel with REST endpoints
(`/api/projects/{id}/agent-browser`), makes startup async and race-safe, and
extends the browser MCP wiring to the Codex provider.

### What works

| Capability | Status |
|---|---|
| Real headed Chrome in each project's unprivileged LXD container | ✅ |
| User watches + logs in by hand via noVNC (passwords/2FA never typed by agent) | ✅ |
| Agent drives the same session over loopback CDP (`browser_*` MCP tools) | ✅ |
| Persistent profile — logins survive container restarts | ✅ |
| Only noVNC (`6080`) exposed, behind the Google-auth edge; CDP loopback-only | ✅ |
| Works for both Claude and Codex providers | ✅ (branch) |
| Race-safe async start with status polling | ✅ (branch) |

### The original four gaps

| # | Gap | Detail |
|---|---|---|
| 1 | **Agent sees the DOM, not the page** | The `browser` skill playbook is snapshot-first: the agent reads the accessibility tree. Canvas, charts, maps, image content, visual layout breakage, and CAPTCHAs are effectively invisible. "View everything like a human" requires screenshots as a first-class perception channel. |
| 2 | **The stack is monolithic and immortal** | `gui-up.sh start` launches Xvfb + openbox + Chrome + x11vnc + websockify as one unit, and only an explicit user click ever stops it. The VNC layer runs (RSS + frame-encoding CPU) even when no human is watching, and an abandoned session runs forever. |
| 3 | **The agent cannot self-start the browser** | The skill instructs: *"Ask the user to open the Browser pane (that starts the session), then retry."* Agent-side capability depends on a human UI gesture. |
| 4 | **Resource caps punish the wrong process** | Older browser setup applied CPU/memory limits to the **whole container**, throttling the user's builds and dev servers in order to contain the browser — instead of capping the browser's own process tree. |

## Where we want to go — acceptance criteria

The capability is **done** when every box below is checked. Each criterion is
testable; the phase that delivers it is noted.

### A. Autonomy

- [x] **A1.** When a prompt runs with the `browser` skill selected and the
  browser stack is down, the agent's first `browser_navigate` succeeds
  without any user action — the provider launch path brings the core stack
  up automatically. *(Phase 1)*
- [x] **A2.** SKILL.md contains no instruction to ask the user to open the
  Browser pane in order to *start* the browser (only to *log in*). *(Phase 1)*

### B. Human-like perception

- [x] **B1.** The agent's MCP config enables vision capabilities
  (`--caps=vision`), so coordinate-based, screenshot-grounded interaction is
  available. *(Phase 3)*
- [x] **B2.** The skill playbook prescribes a hybrid perception loop:
  accessibility snapshot for structure/refs, screenshot whenever the task is
  visual (layout verification, canvas/charts, image content) and after
  actions with visual consequences. *(Phase 3)*
- [ ] **B3.** Given a page whose meaning is only visible in pixels (e.g. a
  rendered chart), the agent can correctly answer a question about it. *(Phase 3)*

### C. Human-like action

- [ ] **C1.** All agent input reaches pages as trusted events (CDP-dispatched
  — already true via Playwright) and WebGL is functional (no
  dead-`--disable-gpu` bot tell): `chrome://gpu` reports a software GL
  renderer, and `webglreport.com`-style probes succeed. *(Phase 4)*
- [ ] **C2.** An OS-level input fallback (xdotool against the Xvfb display)
  exists and is documented in the skill, for sites that reject
  CDP-originated input. *(Phase 4)*
- [x] **C3.** The skill playbook includes pacing rules (wait for load states,
  no machine-gun action sequences). *(Phase 4)*
- [ ] **C4.** The agent never types user credentials; login remains a manual
  handoff through the pane. *(invariant — holds today, must keep holding)*

### D. Minimal resources

- [ ] **D1.** **Idle cost is zero:** a browser stack with no agent activity
  and no viewer for the idle TTL (default ~20 min) is reaped automatically;
  `AgentBrowserRunning` reports false afterwards. *(Phase 2)*
- [x] **D2.** **Watching is pay-per-view:** x11vnc + websockify run only
  while a human viewer is (or recently was) connected. Closing the drawer
  tears the view layer down without disturbing Chrome or the agent's
  session. *(Phase 1)*
- [x] **D3.** **Caps hit the browser, not the project:** the browser process
  tree runs under its own cgroup budget (`MemoryMax`≈1.5G, `CPUQuota`≈200%),
  and the container-wide `limits.cpu`/`limits.memory` set by the browser
  feature are removed. A heavy page cannot starve builds; a build cannot be
  throttled because the browser feature exists. *(Phase 2)*
- [x] **D4.** Chrome runs with a resource-diet flag set
  (`--renderer-process-limit`, background networking/translate/etc.
  disabled, audio muted). *(Phase 2)*
- [ ] **D5.** Restart-after-reap is cheap: profile persists, cold start back
  to `ready` in seconds, logins intact. *(Phase 2 — verified behavior)*

### E. Session continuity (already delivered — must not regress)

- [ ] **E1.** User logs in by hand once; agent reuses that session across
  prompts, container restarts, and idle reaps.
- [ ] **E2.** Only port `6080` is reachable from outside the container,
  through the auth-gated dev-URL proxy; CDP `9222` stays loopback-only.

### F. Observability

- [x] **F1.** Per-project browser stack state (status, uptime, last
  activity) is queryable via the REST status endpoint. *(Phase 2/5)*
- [ ] **F2.** Reap events and stack RSS are logged/measurable, so the real
  per-session budget is known rather than estimated. *(Phase 5)*

## Resource model (target)

| State | What runs | Approx. footprint |
|---|---|---|
| Idle (no activity > TTL) | nothing | **0** (profile on disk only) |
| Agent driving, nobody watching | Xvfb + openbox + Chrome + CDP | ~60 MB (X stack) + 0.6–1.2 GB Chrome, capped at 1.5 GB / 2 cores by its own cgroup |
| Agent driving + human watching | above + x11vnc + websockify | + ~50–80 MB, encoding CPU only while frames change |

Live verification still needs to prove the exact footprint and reap timing on
the deploy host, but the implementation now has separate idle, core-only, and
view-attached modes.

## Explicit non-goals (for this roadmap)

- **Residential/user-local egress.** The browser exits via the container's
  datacenter IP; strict providers may challenge, which the manual-login
  handoff absorbs. Per-user egress is a separate future decision.
- **Replacing noVNC with a CDP screencast pane.** Attractive (kills the whole
  VNC layer), but it is a pane rewrite and Xvfb must remain for headed
  Chrome regardless. Deferred behind Phase 5 measurement — see
  [agent-browser-phases.md](agent-browser-phases.md#phase-5).
- **CAPTCHA solving / detection-evasion beyond honest fingerprint hygiene.**
  The design goal is a *real* browser that behaves like one — not an evasion
  toolkit. Logins and challenges stay with the human.
