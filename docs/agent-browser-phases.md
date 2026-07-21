# Agent Browser: Implementation Phases

The phased plan that delivers the acceptance criteria in
[agent-browser-roadmap.md](agent-browser-roadmap.md). Each phase is
independently shippable and leaves the system working; the order is chosen so
that the biggest resource wins and the autonomy fix land first, and the
speculative work lands last (behind measurement).

| Phase | Name | Delivers criteria | Size |
|---|---|---|---|
| 0 | Land `codex/simplify-agent-browser` | foundation | merge |
| 1 | Decompose the stack: `core` vs `view` | A1, A2, D2 | M |
| 2 | Idle reaper + right-sized caps | D1, D3, D4, D5, F1 | M |
| 3 | Human-like seeing (vision) | B1, B2, B3 | S |
| 4 | Human-like acting | C1, C2, C3 | S–M |
| 5 | Measure, then (maybe) the big swing | F2, (opt.) D2+ | S + spike |

---

## Phase 0 — Land `codex/simplify-agent-browser`

**Goal:** merge the branch; everything below builds on it.

What it brings (already reviewed):

- WebSocket lifecycle channel (`/ws/browser-gui`) → REST at
  `/api/projects/{id}/agent-browser` (`GET` status / `POST …/start` /
  `DELETE` stop) in
  [project_handler.go](../backend/internal/transport/http/handlers/project_handler.go).
- Async, race-safe start: per-project state map + monotonic start counter in
  [service.go](../backend/internal/service/project/service.go)
  (`StartAgentBrowser`, `AgentBrowserStatus`, `StopAgentBrowser`);
  provisioning runs in a background goroutine while the frontend polls.
- `AgentBrowserRunning` container probe → idempotent starts.
- Codex provider gets the browser MCP (inline `-c mcp_servers.browser.*`
  flags) on par with Claude's `--mcp-config`.
- Legacy browser GUI naming → `AgentBrowser` naming throughout; handler +
  provider tests.

**Exit:** branch merged to `main`, deploy green.

---

## Phase 1 — Decompose the stack: `core` vs `view`

**Goal:** the agent needs Chrome; the human needs pixels. Stop coupling them.
Fixes gaps #2 (partially) and #3; delivers **A1, A2, D2**.

### Design

Split the process tree managed by
[gui-up.sh](../backend/internal/integration/containers/templates/gui-up.sh) into
two named groups:

- **`core`** — Xvfb `:99` + openbox + headed Chrome (persistent profile,
  loopback CDP `:9222`). Required whenever the *agent* drives. Headed-on-Xvfb
  is deliberate and stays: real rendering, real window, no
  `HeadlessChrome` fingerprint.
- **`view`** — x11vnc + websockify/noVNC `:6080`. Required only while a
  *human* is watching or logging in.

### Changes

| Where | What |
|---|---|
| `templates/gui-up.sh` | New verbs: `start-core`, `start-view`, `stop-view`, plus `status` reporting each layer separately (e.g. `core=ready view=off clients=0`). Plain `start`/`stop` keep working as "everything" for compatibility. |
| [agent_browser.go](../backend/internal/integration/containers/agent_browser.go) | `EnsureAgentBrowser(ctx, name, layer)` — or split into `EnsureAgentBrowserCore` / `EnsureAgentBrowserView` + `StopAgentBrowserView`. Readiness for `core` = CDP responds; for `view` = noVNC responds. |
| Provider launch paths ([claude/command.go](../backend/internal/agent/claude/command.go), [codex/command.go](../backend/internal/agent/codex/command.go)) | When `req.EnableBrowser`: after `EnsureAgentBrowserMCP`, also `EnsureAgentBrowserCore`. **The agent self-starts the browser.** |
| [service.go](../backend/internal/service/project/service.go) | `StartAgentBrowser` (pane) ensures `core` + `view`. New `StopAgentBrowserView` used by the drawer-close path; full `StopAgentBrowser` stays behind the explicit stop button. |
| REST handler | `DELETE /agent-browser?scope=view` (or `POST …/view/stop`) so the frontend can drop only the view layer. |
| Frontend ([useAgentBrowserSession.ts](../frontend/src/hooks/chat/useAgentBrowserSession.ts)) | On drawer close (`enabled` → false): call the view-stop endpoint. Status hook keys off the combined status payload. |
| [SKILL.md](../backend/internal/integration/containers/templates/skills/browser/SKILL.md) | Delete "*ask the user to open the Browser pane, then retry*". Replace with: the browser starts with the skill; direct the user to the pane **only for logins**. |

### Acceptance

- A prompt with the `browser` skill on a cold container: first
  `browser_navigate` works, `view` layer never started (**A1**).
- Open pane → `view` comes up on the *same* Chrome (cookies visible); close
  drawer → x11vnc/websockify gone within seconds, agent session undisturbed
  (**D2**).
- SKILL.md contains no start-the-pane instruction (**A2**).

### Risks

- Provider launch path now blocks on core-start (~5–15 s cold). Mitigate:
  reuse the Phase 0 async pattern — kick off ensure at run-start; the MCP
  server retries CDP attach, and first `browser_navigate` typically lands
  after readiness anyway.
- Drawer close vs. user just reloading the page: add a short grace (e.g.
  view-stop only after N seconds without a status poll, or rely on the
  Phase 2 in-container client count) to avoid flapping.

---

## Phase 2 — Idle reaper + right-sized caps

**Goal:** idle cost → zero; caps scoped to the browser, not the project.
Fixes gaps #2 (rest) and #4; delivers **D1, D3, D4, D5, F1**.

### 2a. Backend idle reaper

- Extend the Phase 0 state map in
  [service.go](../backend/internal/service/project/service.go) with
  `lastActivity time.Time`, bumped by: agent runs launched with
  `EnableBrowser`, `StartAgentBrowser`, and pane status polls.
- One `time.Ticker` goroutine on the service: every minute, for each project
  whose stack is `ready` and `lastActivity` older than the TTL (**default
  ~20 min**, configurable), call `StopAgentBrowser` and log a reap event.
- The profile persists in `/workspace/.browser-gui/profile`, so the reap is
  safe: next use cold-starts back to `ready` in seconds with logins intact
  (**D5**).

### 2b. Authoritative view-idle from inside the container

Drawer-close is a *cooperative* signal; laptops get shut. Make
`gui-up.sh status` report the x11vnc client count (x11vnc logs connects /
disconnects; simplest robust probe: `ss -tn state established
'( sport = :5900 )' | wc -l`). The reaper (or a lightweight in-container
cron) stops the `view` layer when clients have been 0 for a few minutes.

### 2c. Caps on the browser's own cgroup

- In `gui-up.sh`, launch Chrome under its own scope:

  ```sh
  systemd-run --scope --unit=agent-browser -p MemoryMax=1536M -p CPUQuota=200% \
    "$CHROME" --user-data-dir="$PROFILE" ...
  ```

  (Fallback if systemd scopes are unavailable in the container: keep the
  current direct launch — the flag diet below still helps.)
- Remove `limits.cpu` / `limits.memory` from `EnsureAgentBrowserLimits` in
  [agent_browser.go](../backend/internal/integration/containers/agent_browser.go);
  keep `security.nesting`. Container-wide limits stop existing *because of
  the browser feature* (**D3**). Ship as a migration: unset the two keys on
  next launch for containers that have them.

### 2d. Chrome resource-diet flags

Add to the launch line (**D4**):

```
--renderer-process-limit=4
--disable-background-networking
--disable-features=Translate,MediaRouter,OptimizationHints
--disable-background-timer-throttling=false   # keep default throttling
--metrics-recording-only
--mute-audio
```

### 2e. Status payload (F1)

Extend `AgentBrowserInfo` with `core`, `view`, `viewerCount`, `uptimeSec`,
`lastActivity` so the pane (and ops) can see the real state via
`GET /api/projects/{id}/agent-browser`.

### Acceptance

- Start browser, do nothing, wait TTL → stack gone, status `stopped`,
  reap logged (**D1**). Re-run a browser prompt → `ready` again in seconds,
  still logged in (**D5**).
- `lxc config get <container> limits.memory` is empty on migrated
  containers; OOM-killing a deliberately heavy page kills only Chrome's
  scope, not the user's build (**D3**).

---

## Phase 3 — Human-like seeing (vision)

**Goal:** screenshots become a first-class perception channel. Fixes gap #1;
delivers **B1, B2, B3**. Mostly config + playbook — small and high-leverage.

### Changes

| Where | What |
|---|---|
| [templates/mcp-claude.json](../backend/internal/integration/containers/templates/mcp-claude.json) | `"args": ["@playwright/mcp", "--cdp-endpoint", "http://127.0.0.1:9222", "--caps=vision"]` |
| [codex/command.go](../backend/internal/agent/codex/command.go) | Mirror the same arg in the inline `mcp_servers.browser.args` flag. Keep the two configs byte-equivalent in intent — they are the same server. |
| [SKILL.md](../backend/internal/integration/containers/templates/skills/browser/SKILL.md) | Rewrite the "How to work" playbook as a **hybrid perception loop** (below). |

### The hybrid perception loop (playbook content)

1. `browser_navigate`, then `browser_snapshot` — structure, text, and element
   refs. Still the default: cheap and precise.
2. **Screenshot when the task is visual**: layout verification ("does it look
   right"), canvas/charts/maps, image content, anything the snapshot renders
   as an empty or opaque node, and *after* actions with visual consequences.
3. Act via refs when you have them; use vision/coordinate interaction when
   the DOM lies (custom widgets, canvas UIs).
4. Snapshot or screenshot again to confirm — never assume an action worked.

Keep screenshot cost bounded: JPEG, moderate quality, viewport-only by
default (full-page only when explicitly needed). Viewport stays **1366×768**,
matched between the Xvfb screen and `--window-size` — preserve that invariant
when editing `gui-up.sh`.

### Acceptance

- Vision tools present in the agent's tool list on both providers (**B1**).
- Playbook prescribes when to snapshot vs. screenshot (**B2**).
- E2E probe: a page whose answer exists only in pixels (a rendered chart);
  the agent answers correctly (**B3**).

---

## Phase 4 — Human-like acting

**Goal:** input indistinguishable from a human's, honest fingerprint, sane
pacing. Delivers **C1, C2, C3** (C4 is an invariant, not new work).

### 4a. Fingerprint hygiene (C1)

Playwright-over-CDP already dispatches **trusted** input events — that is
most of "acting like a human". The remaining tell is a dead GPU stack:

- Replace `--disable-gpu` in `gui-up.sh` with software WebGL:

  ```
  --use-gl=angle --use-angle=swiftshader-webgl
  ```

  so `chrome://gpu` shows a software renderer and WebGL probes succeed
  instead of erroring. Verify against a WebGL-requiring page in the
  acceptance pass. (Chrome for Testing's UA is a normal Chrome UA — leave it
  alone.)

### 4b. OS-level input fallback (C2)

For the minority of sites that reject CDP-originated events, add an
**xdotool path** (already installed by the
[base-image script](../backend/internal/integration/containers/baseimage.go)) —
input injected at the X server is indistinguishable from a physical
mouse/keyboard:

- Ship `/workspace/.browser-gui/human-input.sh` alongside `gui-up.sh`
  (same push-templated-file mechanism): thin verbs over
  `DISPLAY=:99 xdotool` — `move x y` (eased, multi-step `mousemove`),
  `click x y`, `type "text"` (with `--delay` cadence), `key <keysym>`,
  `scroll <n>`.
- Document in SKILL.md: *use `browser_*` tools first; if a site visibly
  swallows or rejects those interactions, fall back to
  `sh /workspace/.browser-gui/human-input.sh …` with coordinates taken from
  a screenshot.* (Coordinates map 1:1 — window at 0,0, size = screen size.)

### 4c. Pacing rules (C3)

Add to SKILL.md's write-policy section:

- Wait for load states / `browser_wait_for` between navigation and action.
- One action, then observe; no blind multi-action bursts.
- Human-scale rhythm on authenticated sites — this is both robustness
  (fewer race failures) and courtesy to the platform.

The existing **write-approval policy** (confirm before posting/DMing/buying)
and **never type credentials** (C4) carry over verbatim.

### Acceptance

- WebGL probe passes; `chrome://gpu` shows software GL (**C1**).
- Demo: an interaction that fails via `browser_click` on a
  CDP-hostile test page succeeds via `human-input.sh` (**C2**).
- SKILL.md contains the pacing section (**C3**).

---

## Phase 5 — Measure, then (maybe) the big swing

**Goal:** know the real budget; only then consider replacing the VNC layer.
Delivers **F2**; optionally extends D2.

### 5a. Measurement (do this)

- Log per-stack RSS at reap time and on a slow ticker:
  `smem`-style sum over the stack's PIDs (or the systemd scope's
  `memory.current` from Phase 2c) — Chrome, Xvfb, x11vnc, websockify
  reported separately.
- Count: starts, reaps (idle vs. explicit), mean session lifetime, share of
  sessions where a human ever attached the view layer.
- Success metric for the phases above: **host-wide browser RSS ≈ (number of
  *currently active* sessions) × (bounded per-session cost)** — no long tail
  of forgotten stacks.

### 5b. Optional spike: CDP screencast pane (only if numbers still hurt)

Replace x11vnc + websockify + noVNC with a backend-proxied CDP
`Page.startScreencast` → WebSocket → `<canvas>` in the drawer, using
`Input.dispatchMouseEvent`/`dispatchKeyEvent` for the user's login clicks.

- **Pro:** kills the entire VNC layer permanently (~50–80 MB + encode CPU per
  watched session), sharper image, lower latency, and the pane no longer
  needs port `6080` at all (CDP stays loopback; the backend proxies).
- **Con:** a rewrite of the pane + a new backend proxy; Xvfb must stay
  regardless (headed Chrome needs a display); careful input-event mapping.
- **Decision rule:** only pursue if Phase 1/2 telemetry shows the view layer
  is still a material cost. Otherwise close the roadmap at Phase 4.

---

## Invariants to preserve in every phase

1. **One shared Chrome** per project — user view and agent CDP attach to the
   same session (that's the whole point).
2. **Credentials never pass through the agent**; logins happen by hand in
   the pane (roadmap C4/E1).
3. **Only `6080` reachable from outside**, behind the auth-gated dev-URL
   proxy; CDP `9222` loopback-only (roadmap E2).
4. **Profile lives under `/workspace`** and survives restarts, reaps, and
   container recreation.
5. `gui-up.sh` and install recipes stay **idempotent** and re-push only on
   content change (sha256 marker pattern).
