# Timestamped walkthrough and English chapter translation

This is a chapter-level English translation and feature map, not a word-for-word substitute for the [Arabic transcript](transcript/application-walkthrough-ar.vtt).

| Time | What happens | Features established |
| --- | --- | --- |
| 00:00–00:28 | The narrator introduces Remote as a collection of small computers. Each computer is a project. A new project named `youtube` is created to contain all YouTube-related work. | Project-as-computer model, project creation, project naming, project-specific organization |
| 00:28–00:56 | The older project is deleted and the new one provisions. Project Info and Settings are opened while it starts. | Provisioning state, project deletion, project inspection, container settings |
| 00:56–01:20 | Global Agent settings show Claude, Codex/ChatGPT, and Kimi authentication. Codex and Kimi are signed in; Claude is not yet configured. | Provider authentication, host credential reuse, subscription-based agents |
| 01:20–01:54 | Project settings show CPU, memory, disk quota, defaults, and lifecycle controls. | Resource governance, start/stop/restart/delete |
| 01:54–02:35 | A second project, `arabic tech`, is created. The narrator emphasizes that it is a completely separate computer from `youtube`. | Multiple isolated projects on one server |
| 02:35–03:19 | A new chat is opened. The narrator explains that chats can use Codex, Claude, or Kimi and can configure model, thinking, speed, mode, and skills. Codex GPT-5.6 Sol, XHigh thinking, Auto speed, Code mode, and the browser skill are shown. | Multi-provider chat, per-chat agent configuration, skill picker |
| 03:19–03:58 | “Hi, who are you?” is sent in both projects. Both chats work at once. The narrator says many projects and chats can run concurrently. | Concurrent agents, independent project state, activity indicators |
| 03:58–04:47 | A longer Arabic prompt asks the agent to prepare for YouTube links and build a page containing each video and its transcript. Voice capture is visible. | Arabic/English interaction, voice dictation, app-generation workflow |
| 04:47–05:34 | The narrator explains why the agent calls itself a computer: it can inspect the Linux environment, install tools, and run software rather than only answer in a web chat. | Full tool-capable Linux runtime |
| 05:34–07:18 | A second, unrelated marketing-site job is dictated in another project while the YouTube task keeps running. Follow-up prompts can be queued during active work. | Parallel work, background jobs, queued prompts, project isolation |
| 07:18–08:21 | The YouTube agent reports that the page is ready. The development server is repaired and opened in the embedded Browser panel. | Tool installation, persistent dev server, port discovery, embedded app preview |
| 08:21–08:46 | The user visually selects text in the preview and asks the agent to make it more interesting. The composer contains captured text, HTML, layout, and style context. | Visual inspector, inspect-to-prompt, vibe-coding iteration |
| 08:46–09:24 | The narrator explains automatic URLs, multiple exposed ports, and authenticated sharing. The generated site is also opened in a regular browser tab. | Automatic preview routing, external URL, login wall, project sharing |
| 09:24–10:00 | Git history is opened; there is not yet a repository, so the agent is told to create one and commit all updates. | Git initialization through chat, history/diff/switch UI |
| 10:00–11:38 | A full code-server/VS Code IDE is opened in the browser. Source files and extensions are shown. Activity Monitor is used to argue that local desktop agents consume more memory than server-side execution. | Browser IDE, code editing, source control, remote-compute positioning |
| 11:38–11:54 | The updated application is reviewed again in the live preview. | Live refresh, chat-and-preview split view |
| 11:54–12:40 | Agent Browser starts a live Chromium session in the remote Linux machine. It searches for the public IP; the user can intervene for consent/CAPTCHA. | Headed agent browser, shared human/agent control, server-side browsing |
| 12:40–13:21 | German localization confirms the server’s region. Fast.com reports about 2.9 Gbps, illustrating that browsing uses the server’s network rather than the local connection. | Remote geolocation, VPS network path, browser automation performance |
| 13:21–13:49 | The narrator says the browser skill can research the web and use logged-in social-media sessions. | Browser skill, persistent authenticated browsing, research/analysis use cases |
| 13:49–14:36 | Terminal and Files controls are discussed. The file manager shows the complete workspace and download actions without requiring the IDE. | Web terminal, file search/tree/download, non-developer artifact access |
| 14:36–15:00 | The narrator says Remote can be closed and reopened on a phone because it is a PWA; server-side jobs continue. | Mobile/PWA access, background execution |
| 15:00–16:21 | The common chat interface is revisited. The narrator highlights switching between Codex, Claude, and Kimi and asks the agent to build a transcription skill. | Provider switching, reusable/custom skills, project-specific automation |
| 16:21–17:00 | The isolation model is summarized: each project is a different Linux/LXD computer, so the agent can receive broad permissions without sharing a machine with other projects. | Container isolation, per-project blast radius |
| 17:00–17:41 | Project Secrets are opened. The narrator explains storing account/API credentials as environment secrets outside normal workspace files. | Project-scoped secrets, multi-line values, environment injection |
| 17:41–18:23 | The narrator closes with personal use cases: motion graphics, video editing through skills, and uploading finished work to YouTube. | Media automation, end-to-end outbound workflows, delegated credentials |

## Narrative flow

The demonstration is structured as a progressive proof:

1. establish the project/container model;
2. connect multiple agent providers;
3. show concurrent chats and multilingual prompting;
4. build a real application and preview it;
5. revise it visually;
6. expose conventional developer tools;
7. extend the agent into browser automation;
8. finish with persistence, mobile access, isolation, and secrets.

