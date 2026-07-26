# Fable analysis — feature review from the 2026-07-22 demo recording

Analysis of `remote.futrx` ("Remote") derived from the screen recording
`/Volumes/T7/recordings/Screen Recording 2026-07-22 at 1.40.47 PM.mov`
(18 m 23 s, 2974×1500, recorded 2026-07-22 1:40 PM).

## Method

- **Audio**: extracted and transcribed with whisper.cpp (`large-v3-turbo`). The narration is
  Egyptian Arabic (auto-detected, p = 0.94) with occasional English prompts dictated to agents.
  A few stretches collapse into whisper hallucination loops (mostly while the narrator dictated
  English prompts slowly); the frame review covers those gaps visually.
- **Frames**: 221 frames extracted at one frame per 5 seconds and every frame reviewed
  (four parallel reviewers, one per ~4.6-minute segment). Selected frames were re-extracted at
  the full 2974-px resolution to resolve small text.
- Everything below is grounded in what is visible on screen or said in narration; claims that
  exist only in narration are flagged as such.

## What the application is

**Remote** (instance: `remote.futrx.xyz`; the app calls itself "Remote" in its own settings) is a
self-hosted browser workspace for running coding agents — **Codex, Claude, and Kimi** — where
every *project* is a full, isolated Linux computer (an LXC/LXD container: Ubuntu 24.04,
Node 22, agent CLIs and code-server pre-installed). Users create projects, open any number of
chats inside them, and let agents work with **full tool access** inside
`/var/lib/remote/projects/<slug>/workspace`, inspecting the results through an embedded
terminal, file manager, Git history, VS Code (code-server) IDE, live app previews on public
per-port URLs, and a server-side headed Chrome ("Agent Browser") streamed over noVNC.

The demo runs on a small Hetzner VPS in Germany (egress IP `128.140.105.131`, fast.com
measured 2.8 Gbps from inside the container) while the narrator is in Canada, and the client
side is nearly free: the desktop wrapper process shows **59.8 MB** in Activity Monitor next to
WebStorm's 3.18 GB.

## Headline capabilities shown in the video

1. Projects as disposable full computers — create, resize (CPU/RAM/disk), start/stop/restart,
   delete; unlimited projects and chats, three agent runs shown working concurrently.
2. One chat UI over three interchangeable agent CLIs (Codex / Claude / Kimi) with per-prompt
   model, thinking, speed, and mode selectors; mid-chat provider switching.
3. Voice dictation (mixed Arabic + English), prompt queueing, message rewind, chat forking.
4. Skills system — per-provider skill sets, a project-scoped `browser` skill, and the agent
   *building a new skill for itself* on request (skill count visibly increments).
5. Workspace tooling without leaving chat: terminal, file manager with downloads, Git history
   with checkout, element-picker browser preview that pastes computed CSS/HTML into prompts.
6. Agent Browser: a 24/7 server-side Chrome the user and agent share — the user logs in and
   solves CAPTCHAs once, the agent then acts inside the authenticated session.
7. Public preview URLs per app port (`https://<project>--<port>.dev.remote.futrx.xyz`) gated by
   project membership; sharing by email with Google sign-in.
8. Secrets vault per project — values passed to the agent CLI as `--env` per run, never written
   to the container filesystem and (per narration) never shown to the LLM.
9. Host-level agent authentication — sign in once (`codex login --device-auth`,
   `claude auth login --claudeai`, `kimi login`) and credentials seed every container.
10. Progressive Web App usable from a phone; the platform's own UI stays tiny because all
    compute lives server-side.

## Files

| File | Contents |
| --- | --- |
| [01-video-walkthrough.md](01-video-walkthrough.md) | Chronological timeline of the demo with timestamps, narration, and on-screen evidence |
| [02-feature-inventory.md](02-feature-inventory.md) | Exhaustive feature catalog organized by functional area |
| [03-transcript.md](03-transcript.md) | Cleaned bilingual transcript of the Arabic narration |
| [04-marketing-comparison.md](04-marketing-comparison.md) | Marketing one-pager + comparison vs Claude Code, Codex app, OpenClaw, and Hermes Agent |
