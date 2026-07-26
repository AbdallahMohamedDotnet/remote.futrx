# Video walkthrough — chronological timeline

Timestamps are video time (mm:ss). Narration is paraphrased from the Arabic; on-screen text is
quoted verbatim.

## 0:00–0:30 — Introduction: "each project is a computer"

The recording opens on the dark-themed workspace: projects sidebar on the left
("Workspace / Projects", search box "Search projects and chats", counter "1 project - 3 chats"),
a chat titled "who are you" with subtitle "Codex · Ready", and the composer with agent tabs
**Codex | Claude | Kimi**, "Model: Auto", "Skill set — Codex", plus "Thinking: Auto",
"Speed: Auto", "Mode: Code" dropdowns.

Narration: *"I want to show you everything you can do with Remote. Fundamentally, Remote is a
collection of small computers — each computer is a project."* He creates a new project via the
browser prompt ("remote.futrx.xyz says — Project name?") and names it **youtube**: *"anything
related to my YouTube work will live here."* The project appears in the sidebar with a
provisioning spinner.

## 0:30–0:55 — Deleting the old project; container Info page

While youtube provisions, he opens the old project's settings (tabs **Info, Settings, Secrets,
Sharing**). The Info page shows the container's live state: badge "running", Container
`bashmohandes-mazen-channel`, State `RUNNING`, PID `89107`, Processes `24`, Image
"futrx remote dev base: ubuntu 24.04 + node 22 + claude-code + code…" (truncated in the UI),
Architecture `x86_64`, Boot autostart `yes`, created/last-used timestamps, OS
`Ubuntu 24.04.4 LTS`. He deletes the project (transient red "Deleting..." state); the sidebar
count drops to 1 project. Narration: *"I'll delete the old one — I want to start fresh."*

## 0:55–1:20 — Preferences: theme and host agent authentication

**Preferences / Settings** page, tabs **Appearance, Agents, Users, Info**:

- *Appearance*: theme segmented control **System | Dark | Light** with instant "✓ Saved".
- *Agents* — "Manage host authentication for coding agents", one row per provider:
  - **Claude** — "not configured"; "Starts `claude auth login --claudeai` on the host…
    tokens land in `~/.claude/.credentials.json` and seed into every container."
  - **Codex** — "✓ ChatGPT signed in"; "Starts `codex login --device-auth`… signs in with
    ChatGPT so Codex uses subscription limits, not API-key billing."
  - **Kimi** — "✓ Subscription signed in"; "Starts `kimi login`… device code — no API key,
    billed against your membership quota."

Narration confirms: signed in with his ChatGPT/Codex account and Kimi; Claude not yet.

## 1:20–2:00 — Per-project resource limits

Back in the (now provisioned) youtube project's **Settings** tab: stat tiles "Effective CPU: 6",
"Effective memory: 4GiB", "Effective disk quota: No quota", "Server total memory: 3.72 GB";
editable **CPU cores** (1–256), **Memory**, **Disk quota** fields with the warning "Changes
apply live. Lowering memory can stop container processes… Leave a field blank to inherit the
fleet default", plus "Reset to defaults" / "Save limits" and lifecycle buttons **Start / Stop /
Force restart / Delete project**. Narration: *"I can decide how much CPU, max RAM, and disk
quota each project gets — this server is tiny (3.7 GB total) and shared by several computers."*

## 2:00–2:45 — Second project; unlimited chats concept

He creates a second project **arabic tech** (*"it'll build a new site for the Arabic Tech
courses"*). Narration: *"this is one full computer, and this is another full computer… inside a
project I can open any number of chats, and every chat can work with any of them — Codex,
Claude, or Kimi — at the same time. There is literally no limit on the number of running
projects or chats."* A new chat's empty state reads: "**Start a conversation** — The selected
agent runs with full tool access in `/var/lib/remote/projects/youtube/workspace`. Drop, paste,
or upload files to reference them," with the input showing "Connecting..." until the session is
ready.

## 2:45–3:30 — Model, thinking, and skills selection; first prompts

In the composer he sets **Model: GPT-5.6 Sol** (from "Auto"), **Thinking: XHigh**, keeps
Speed: Auto and Mode: Code — narration compares the controls to "what you find in Claude Code
desktop and Codex desktop". He opens the **Skill set** popover ("Search Codex skills"): one
skill loaded, **browser**, tagged **PROJECT** — "Drive a real web browser the user logs into —
open pages, read content, search, fill forms, click, and act in the user's authenticated
sessions (social…)". *"I can add more skills — more on that later."*

He sends "hi who are you" in youtube; the chat header flips to "Codex · Working", the sidebar
chat renames itself to the prompt, shows a `gpt-5.6-sol` badge and elapsed-time counter, and
the composer switches to "Queue next prompt while the agent is working" with a red stop button.
He opens a second chat in arabic tech simultaneously — **two agents running in parallel in
separate containers**.

## 4:00–4:50 — Voice dictation; the YouTube-transcripts task

He clicks the microphone: a "**Listening...**" pill with a red pulsing dot appears over the
composer and live transcription lands in the input — first Arabic, then mixed Arabic/English in
one utterance. The dictated task (youtube project): *"I'll give you some YouTube links and I
want you to make me a web page where I find the YouTube video and its transcription. Prepare
everything and tell me when you're ready for the links."*

## 4:50–5:40 — Why a container matters; tool-call transparency

While Codex works, the reply streams in Arabic with a "thinking" chip and collapsible tool
rows. Expanding "1 tool used" reveals the exact command
(`$ /bin/bash -lc "pwd && rg --files -g '!node_modules' …"`) and its output (`/workspace`,
`scripts/browser.mjs`). Narration: *"ChatGPT on the web can't do this — it can't install
programs from A to Z. Look at the first thing it did: it checked node_modules, confirmed where
it is, saw the space is empty, and decided on a light app. In short: it's inside a real Ubuntu
Linux computer and can do everything it wants."*

## 5:40–7:15 — Three agents in parallel; English dictation

He dictates (in English, *"I'll speak English to get things done"* — and notes you can use any
language) a second task into **arabic tech**: "Build app. Lightweight. Marketing website. For
the community… fetch the community and just collect all the information and build a marketing
website for it. arabic.tech/community". Then a third chat: "Uh, I need to create a promotional
video for. This website. https://arabic.tech/community — What tools will you need to download
for this?" At 7:10 **three chats spin simultaneously across the two containers**. He also
queues a follow-up prompt while the agent is still working (queue clock icon highlights).

## 7:20–8:10 — Results; embedded Browser panel; code-server

The youtube chat finishes: "جاهز دلوقتي ✅" ("ready now") with a clickable link "افتح صفحة
الفيديوهات" ("open the videos page") — one tool row is flagged with a red alert icon (a failed
run, surfaced honestly). He opens the **Browser panel** (right split): header "Browser ●" with
a process/port target dropdown reading "systemd · :8842", later switched to "node · :4173".
The 8842 target renders **code-server** ("futrx-remote-dev-builder", toast "code-server
v4.129.0 has been released!") with the workspace tree (`.agents`, `.browser`, `.claude`,
`.codex`, `scripts`, `app.js`, `index.html`, `server.js`, `styles.css`, `videos.js`).

He then types "the dev server is not running"; the agent replies it will check and restart it
in a persistent way, and the expanded "3 tools used" shows real diagnostics:
`ss -ltnp | rg ':4173'`, `ps aux | rg '[n]ode server.js|[n]pm start'`,
`curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:4173/`. Final message: "Fixed—the
server is now running persistently and returning HTTP 200." with an "Open the page" link.

## 8:10–9:10 — The generated site; element picker; public URL

Full-screen preview of the generated RTL Arabic site "**فيديوهاتك**" (YouTube-transcript
viewer: red play-button brand, "جاهز للمشاهدة" live badge, hero "الفيديو والنص المكتوب في مكان
واحد.", empty-state "مستنيين أول فيديو", feature pills "مناسب لكل الشاشات / بحث داخل النص /
توقيتات قابلة للضغط").

Using the Browser panel's **element picker**, he clicks the site header and then the `<h1>` —
each click pastes a `[Browser element]` block into the composer containing the element's
computed CSS (fontSize 52.15px, fontWeight 700, lineHeight 59.97px, color …) and outer HTML —
and sends "update that tet to be more intersting". The agent rewrites the headline ("اتفرّج.
اقرأ. وافهم أكتر. كل لحظة في مكانها.").

The same app is then shown in a real Chrome window at the public URL
**`https://youtube--4173.dev.remote.futrx.xyz`** — narration: *"it's served on the domain I set
up; servers get ports automatically and it can serve an unlimited number. Can anyone reach it?
No — they need access in the system."*

## 9:10–9:45 — Sharing; Git request

**Sharing** tab of project settings: "Sharing — Control which registered users can access this
project." Card "Project access — 1 project member": add-by-email input + Add button; member
list (futrxofficial@gmail.com) with remove ×. Caption: "Members can use this project —
terminal, chats, secrets, uploads, browser. To add someone here they must first appear in the
global Users panel (Account → Users)." Narration: *"you add the person's name, and since you
already set up the Google stuff, they sign in with Google."*

He checks the **History** panel — "No git repos / No commits / No diff" with a "Switch"
(checkout) button — then asks: "make sure all our updates in a git repo".

## 10:05–10:40 — Full IDE; agent commits

"Open in IDE" opens **code-server** (VS Code web) in its own tab: Welcome screen, workspace
tree, then git initializes live — status bar "main*", Source Control badge 8, and an agent
activity indicator "**Codex (now)**" in the status bar. `index.html` shows the generated app
(`<html lang="ar" dir="rtl">`, title "فيديوهاتك | مشاهدة ونسخ مكتوب", Alexandria Google Font,
theme #0d1117). Narration: *"a full IDE — a code-server copy… literally everything you can
imagine is here"* and, on skills: *"in my personal Remote I have a skill that makes the agent
treat me as a no-code user; if I want code, I open the IDE."*

Back in chat, the agent reports: "Done. All project updates are committed in a clean local Git
repository." — Branch `main`, Commit `6f38630 Build YouTube transcript viewer`, working tree
clean, environment-only files excluded via `.gitignore`.

## 10:55–11:40 — Resource-footprint comparison (macOS Activity Monitor)

He opens Activity Monitor (48 GB Mac): **WebStorm 3.18 GB**, Codex (Renderer) 739 MB, ChatGPT
708 MB, codex CLI 671 MB, Claude ~250 MB — versus the **remote.futrx.dev desktop app at
59.8 MB**. Narration: *"WebStorm alone takes three gigs… this took 60 MB. I could run five
thousand dev servers — I don't need a heavy laptop anymore; the entire programming workflow
happens in the browser."*

## 11:55–13:25 — Agent Browser: a shared, server-side, always-on Chrome

In the Browser panel he flips the key-icon toggle: header becomes "**Agent browser** — Live
login session · starting…" with the message "log in to your site once it loads; the agent
shares this session." A **noVNC** stream connects ("Connected (encrypted) to youtube.99…")
showing **Chrome for Testing v148.0.7778.96** running inside the container.

He googles "what is my ip": Google serves a reCAPTCHA ("Select all images with a bus") which he
solves **manually inside the shared session** — narration: *"sometimes it gets annoyed because
it knows you're a bot; you come solve it yourself once and life is good."* The IP resolves to
`128.140.105.131` and Google renders its German consent page — *"it thinks I'm in Germany even
though I'm in Canada, because my computer lives on the Hetzner server in Germany."* fast.com
inside the agent browser measures **2.8 Gbps**: *"your agent browses the internet at your
server's speed."* Narration adds: *"it's like computer-use but much better — it runs 24 hours
without my computer being on… I can tell it to live its life: it searches by itself this way,
and I've had it go through my social media, analyze, and come back."* Stopping the toggle shows
"Agent browser stopped. Toggle it on again to restart."

## 13:45–14:35 — Terminal, Files, downloads

- **Terminal** drawer: "Terminal ● — Connected - /var/lib/remote/projects/youtube/workspace",
  live shell `root@youtube:/workspace#` (he runs `ls -all`); resizable from full overlay to a
  right split. Narration: *"quick checks without even opening the IDE."*
- **Files** panel: "Files / workspace" with "Search all files...", the full tree including
  dotfolders (`.agents`, `.browser`, `.browser-gui`, `.claude`, `.codex`, `.git`, `.vscode`),
  file sizes, and per-row **download** icons for both files and folders. Narration: *"a full
  file manager — more useful for people who don't want to touch code."*

Meanwhile the promo-video chat reports its plan: install **Playwright + Chromium, FFmpeg,
Arabic fonts, optionally Remotion** — "Nothing like Premiere Pro is required… I haven't
installed anything yet." He replies "download whatever you need. and go ahead" — narration:
*"live your life — it has a full computer."*

## 14:35–15:30 — PWA, parallel work, provider switching

Narration: *"I can close this entirely and open it from my phone — it's a Progressive Web App…
I left it working on its own"* (the arabic-tech marketing-site agent is still building — it
scraped real figures from the community page: **17,348 members, 6,750 students, 4.74 average
rating**). On switching providers mid-conversation: *"the power is that I can flip from Codex
to Claude to Kimi in the same chat if I feel one of them is underperforming."*

## 15:30–17:00 — The agent builds itself a new skill

In the youtube chat he sends: "**build a skill, that will let the agent know how to transcribe
videos**". The agent responds (Arabic) that it will create the skill under
**`/workspace/.agents/skills/`**, works through multiple tool calls describing design decisions
(JSON output; separating transcript/captions/translation; distinguishing original vs
auto-generated vs AI text with confidence levels). By 17:30 the composer's **Skill set count
badge increments from 1 to 2** — the self-built skill is registered.

## 17:00–17:30 — Secrets

**Secrets** tab: "Configure environment secrets passed to agents in this project." KEY + value
inputs ("multi-line OK — paste PEM keys, JSON, etc."), Add button, "No secrets yet." Footnote:
"Secrets are passed to the selected agent CLI as `--env KEY=VALUE` on every prompt run. They
never land in the container's filesystem and are not synced back from it." Narration: *"the
secrets stay away from the LLM — it doesn't see them but knows how to call them; you record
them here like in Vercel."*

## 17:30–18:20 — Closing: two months of real use

Narration: *"I've been using Remote for two months for everything imaginable — in my videos I
have it talk to Graphixy for motion graphics; I built myself a set of skills that do the video
editing, and all my recent videos were made through Remote. After editing, it uploads them to
YouTube itself because I gave it YouTube access… The others are trying to build this but
without the same strength, because this is built on complete project isolation — every project
is a full LXC container, a separate Linux computer. You give the LLM every permission it needs…
and it turns into something out of this world. And if a computer breaks I don't care — it's
completely isolated, so it's very, very safe."*

The video ends with both agents still working (the promo-video chat at "8 tools used",
capturing the site in a phone-sized viewport).
