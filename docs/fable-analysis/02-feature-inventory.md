# Feature inventory

Every feature observed in the recording, organized by functional area. "Seen" cites the video
timestamp of the clearest occurrence. Items marked *(narration)* were claimed by the narrator
but not directly demonstrated on screen.

## 1. Product identity and access

| Feature | Details | Seen |
| --- | --- | --- |
| Self-hosted web workspace | Instance at `remote.futrx.xyz`; the app names itself "Remote" ("Choose how Remote looks on this device") | 0:15, 0:55 |
| Desktop app wrapper | macOS process `remote.futrx.dev` — 59.8 MB resident vs WebStorm 3.18 GB, ChatGPT 708 MB, codex CLI 671 MB | 11:05 |
| Progressive Web App | Whole workspace usable from a phone *(narration)* | 14:40 |
| Account footer | Avatar, signed-in email, settings shortcut, sign-out | 0:00 |
| Google sign-in for members | Shared users authenticate with Google *(narration; Sharing UI shown)* | 9:25 |

## 2. Projects — containers as "full computers"

| Feature | Details | Seen |
| --- | --- | --- |
| Project = isolated LXC/LXD container | Ubuntu 24.04.4 LTS, x86_64, image "futrx remote dev base: ubuntu 24.04 + node 22 + claude-code + code…" (name truncated in UI); agent CLIs and code-server pre-installed | 0:35 |
| One-click creation | Sidebar "+" → name prompt → container provisions with a live spinner | 0:15–0:25 |
| Unlimited projects and chats | *(narration: "literally no limit")*; two projects + six chats shown | 3:50 |
| Container Info page | State, PID, process count, image, architecture, boot autostart, created/last-used, OS distribution and kernel; "running" badge; "refreshed Ns ago" + manual refresh | 0:30–0:40 |
| Resource limits, live-applied | CPU cores (1–256), memory, disk quota; "Effective" stat tiles + server total memory; blank = inherit fleet default; warning that lowering memory can stop processes; Reset to defaults / Save limits | 0:40, 1:30, 17:25 |
| Lifecycle controls | Start / Stop / Force restart / Delete project; transient "Deleting..." state | 0:40 |
| Workspace path contract | Agents run with **full tool access** in `/var/lib/remote/projects/<slug>/workspace` (disclosed in every new chat's empty state) | 2:35 |
| Project scaffold | Dotfolders provisioned per container: `.agents`, `.browser`, `.browser-gui`, `.claude`, `.codex`, `.vscode` (+ `.git` after init) | 10:10, 14:00 |

## 3. Chats and agent orchestration

| Feature | Details | Seen |
| --- | --- | --- |
| Three interchangeable agents | Composer tabs **Codex / Claude / Kimi**; switch provider mid-chat *(switch narrated, tabs shown throughout)* | 0:00, 15:10 |
| Parallel agents across containers | Three chats spinning simultaneously across two projects | 7:10 |
| Chat status | Header "Codex · Ready / Working" with status dot; green activity dots on project and chat rows | 3:25 |
| Auto-titling | "New chat" renamed to the first prompt | 3:25, 7:10 |
| Sidebar chat metadata | Model badge (`gpt-5.6-sol`), clock/elapsed counter, spinner while running, done-dot when finished | 3:25 |
| Prompt queueing | While an agent works: "Queue next prompt while the agent is working", queue (clock) button, red stop (cancel) button | 3:25, 7:15 |
| Rewind | Per-user-message hover control to rewind the conversation to that point | 0:45, 8:05 |
| Per-chat row actions | Hover icons: hide (eye-off), fork (branch), delete (X) | 3:55, 17:35 |
| Loose vs project chats | Initial "3 chats" existed with no project attached (loose chats); project chats inherit the container workspace | 0:00 |
| Session startup state | New chat input shows "Connecting..." until the agent session is ready | 2:35 |

## 4. Composer

| Feature | Details | Seen |
| --- | --- | --- |
| Model selector | "Model: Auto" → "GPT-5.6 Sol" (per-provider model list) | 3:00 |
| Thinking selector | Auto → **XHigh** (reasoning-effort levels) | 3:05 |
| Speed selector | "Speed: Auto" | 0:00 |
| Mode selector | "Mode: Code" | 0:00 |
| Skill set picker | Per-provider ("Codex skills" tooltip), count badge, searchable popover | 3:15, 13:35 |
| Skills chip row | Active skills shown as removable chips (e.g. `browser` with PROJECT scope tag) | 13:25 |
| Attachments | "+" button; drop, paste, or upload files; "@ to add files, / for commands" placeholder | 0:00, 2:35 |
| Voice dictation | "Listening..." pill with red pulsing dot; live word-by-word transcription; handles Arabic, English, and mixed utterances | 4:05–7:05 |
| Send / stop | Send arrow ↔ red stop square while running | 3:20 |

## 5. Agent output and message rendering

| Feature | Details | Seen |
| --- | --- | --- |
| Streaming with "thinking" chip | Animated indicator during reasoning | 5:05 |
| Collapsible tool-call groups | "N tools used" accordions expanding to the exact `$ /bin/bash -lc …` command and its stdout | 5:15–5:20, 7:50 |
| Failure surfacing | Red alert badge on tool groups containing a failed run | 7:20, 14:15 |
| Clickable links in replies | "افتح صفحة الفيديوهات", "Open the page" | 7:20, 8:15 |
| Full RTL/bilingual rendering | Egyptian Arabic prose + English structured summaries with inline code chips (`main`, `6f38630`, `.gitignore`) in one message | 10:35 |
| Diagnostics transparency | Agent shows real ops commands: `ss -ltnp`, `ps aux`, `curl -w '%{http_code}'` | 7:50 |

## 6. Skills system

| Feature | Details | Seen |
| --- | --- | --- |
| Per-provider skill sets | Skill picker scoped to the selected agent ("Search Codex skills") | 3:15 |
| Project-scoped skills | `browser` skill tagged **PROJECT**: "Drive a real web browser the user logs into — open pages, read content, search, fill forms, click, and act in the user's authenticated sessions (social…)" | 3:15 |
| Skills live at a filesystem path | `/workspace/.agents/skills/` inside the container | 16:00 |
| Agent-built skills | Prompt "build a skill, that will let the agent know how to transcribe videos" → agent designs and writes the skill; Skill set badge increments 1 → 2 | 15:45–17:30 |
| User-authored skill libraries | Narrator's personal Remote has skills for a no-code persona and full video editing *(narration)* | 10:00, 17:50 |

## 7. Workspace tools (chat header)

### Terminal
- Overlay or right-split drawer; "Connected - /var/lib/remote/projects/youtube/workspace";
  live root shell `root@youtube:/workspace#`; per-chat terminal session ("Connected to hi who
  are you terminal"). Seen 13:50–13:55.

### Files
- "Files / workspace" panel: full tree with dotfolders, file sizes, "Search all files...",
  refresh; per-row **download** for both files and folders. Seen 14:00–14:05.

### History (Git)
- Repository dropdown, commit list, commit detail/diff pane, **Switch** (checkout) button;
  empty states "No repositories / No commits / No diff". Seen 9:40.
- Agent-driven git: "make sure all our updates in a git repo" → repo init + commit
  `6f38630 Build YouTube transcript viewer` on `main`, clean tree, `.gitignore` for
  environment-only files. Seen 9:50–10:35.

### Open in IDE (code-server)
- Full VS Code web (code-server v4.129.0) against the project workspace, in a separate tab or
  the Browser panel; git integration (branch `main*`, SCM badge); **agent activity indicator in
  the status bar** ("Codex (now)" / "Codex (33 seconds ago)"). Seen 7:35, 10:10–10:50.

### Browser panel (app preview)
- Right-split panel with **process/port target picker** ("systemd · :8842", "node · :4173") —
  discovers listening servers in the container; reload, open-external, close; resizable split
  ("Resize browser preview"). Seen 7:25, 9:35, 13:05.
- **Element picker → prompt injection**: clicking an element in the preview pastes a
  `[Browser element]` block (computed CSS + outer HTML) into the composer for precise
  design-edit requests. Seen 8:25–8:45.

## 8. Agent Browser (shared headed Chrome)

| Feature | Details | Seen |
| --- | --- | --- |
| Toggle from Browser panel | Key icon → "Agent browser — Live login session · starting… / connected / stopped" | 11:55, 13:25 |
| Server-side headed Chrome | Chrome for Testing v148.0.7778.96 (`--no-sandbox`) inside the container, streamed via encrypted **noVNC** ("Connected (encrypted) to youtube.99…") | 12:10–12:15 |
| Human-in-the-loop auth | "Log in to your site once it loads; the agent shares this session" — user manually solves Google reCAPTCHA, agent inherits the cleared/authenticated session | 12:20–12:35 |
| Always-on | Runs 24/7 on the server without the user's machine *(narration: "like computer-use but much better")* | 11:55 |
| Server-grade egress | Egress IP 128.140.105.131 (Hetzner, Germany); fast.com **2.8 Gbps** from inside the session | 12:20, 12:55–13:05 |
| Autonomous browsing | Agent searches the web and operates the narrator's social-media accounts through the shared session *(narration)* | 13:30–13:45 |

## 9. App previews and public URLs

| Feature | Details | Seen |
| --- | --- | --- |
| Per-port public URLs | `https://<project>--<port>.dev.remote.futrx.xyz` (e.g. `youtube--4173`) on the operator's domain | 8:50 |
| Unlimited servers/ports | Apps get ports automatically; any number can be served *(narration)* | 8:55 |
| Access-gated previews | Only users with system access (project members) can reach the URLs *(narration; enforced via Sharing)* | 9:05–9:20 |

## 10. Settings and administration

### Project settings (per project: Info / Settings / Secrets / Sharing)
- **Info**: container/OS/resources/network/agent-tooling inspection (see §2). Seen 0:30.
- **Settings**: resource limits + lifecycle (see §2). Seen 1:30, 17:25.
- **Secrets**: KEY/value env secrets ("multi-line OK — paste PEM keys, JSON, etc.");
  "Secrets are passed to the selected agent CLI as `--env KEY=VALUE` on every prompt run. They
  never land in the container's filesystem and are not synced back from it." Narration: hidden
  from the LLM itself; "like Vercel". Seen 17:05.
- **Sharing**: add member by email; members can use "terminal, chats, secrets, uploads,
  browser"; prerequisite: user must exist in the global Users panel (Account → Users);
  member list with remove. Seen 9:20.

### App preferences (Appearance / Agents / Users / Info)
- **Appearance**: theme System/Dark/Light with instant save. Seen 0:55.
- **Agents** (host authentication, done once, seeded into every container):
  - Claude: `claude auth login --claudeai` → `~/.claude/.credentials.json`
  - Codex: `codex login --device-auth` → ChatGPT subscription limits, not API billing
  - Kimi: `kimi login` → device code, membership quota
  Status per provider ("not configured" / "✓ signed in") + sign-in/refresh buttons. Seen 1:00.
- **Users**, **Info** tabs present (not opened in the video). Seen 0:55.

## 11. Navigation and UX

| Feature | Details | Seen |
| --- | --- | --- |
| Projects sidebar | Search across projects and chats; aggregate counter ("2 projects - 6 chats"); per-project gear + "+" (new chat) + expand/collapse; collapsible to a thin icon rail | 0:00, 8:20 |
| Tooltips | "New project", "New chat in this project", "Collapse", "Codex skills", "Resize browser preview" | 2:00, 3:50, 16:35 |
| Three-zone layout | Sidebar / chat column / dockable right tool panel (Terminal, Files, History, Browser) — panels swap in place and resize | throughout |
| Dark theme | Near-black panels, blue accent (selection/primary), green status, red stop/destructive | throughout |
| Scroll-to-bottom | Floating arrow in long chats | 7:40 |

## 12. Demonstrated end-to-end workflows

1. **Idea → running web app**: dictated Arabic prompt → agent scaffolds an RTL Arabic
   YouTube-transcript viewer ("فيديوهاتك", Alexandria font, #0d1117) → served persistently on
   :4173 → public URL → element-picker design iteration → git repo + commit. (4:00–10:35)
2. **Web scraping → marketing site**: agent fetches arabic.tech/community, extracts verified
   proof points (17,348 members, 6,750 students, 4.74 rating), builds an Arabic-first RTL
   marketing site. (5:40–15:00)
3. **Toolchain self-provisioning**: promo-video task → agent proposes Playwright/Chromium,
   FFmpeg, Arabic fonts, optional Remotion → "download whatever you need. and go ahead" →
   installs and starts phone-viewport site captures. (6:50–17:35)
4. **Agent self-extension**: agent writes a new video-transcription skill for itself into
   `/workspace/.agents/skills/`; skill count updates live. (15:45–17:30)
5. **Ops in chat**: "the dev server is not running" → agent diagnoses with ss/ps/curl and
   restarts it persistently, reporting HTTP 200. (7:45–8:15)
6. **Production use** *(narration)*: two months of daily use — motion graphics via Graphixy,
   full video editing via custom skills, and autonomous YouTube uploads with granted access.
   (17:40–18:10)

## Gaps — present in the UI but not exercised in the video

- Preferences → **Users** and **Info** tabs (never opened).
- Project Info page's resources/network/agent-tooling sections below the OS block (never
  scrolled into view; the second visit showed "Loading container data...").
- History panel with an actual commit selected (only empty state + agent-side commit shown).
- The **fork** and **hide** chat-row actions (icons visible, never clicked).
- Claude and Kimi as active providers (Codex ran every demo; Claude was unauthenticated).
- The queue button's queued-prompt management UI (a prompt was queued at 7:15 but the queue
  itself was never opened).
