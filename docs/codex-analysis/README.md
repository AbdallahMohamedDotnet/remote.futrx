# Remote application video analysis

This directory contains a transcript-led, frame-verified analysis of the 18:23 screen recording `Screen Recording 2026-07-22 at 1.40.47 PM.mov`.

The recording is primarily spoken Egyptian Arabic with English product and engineering terms. The analysis covers every feature that is shown or described in the recording. It is a video analysis, not a claim that every hidden API or administrative capability in the source code was exercised.

## Start here

| Artifact | Purpose |
| --- | --- |
| [Feature analysis](feature-analysis.md) | Complete feature inventory, product model, workflows, strengths, limitations, and risks |
| [Walkthrough timeline](timeline.md) | Timestamped English chapter translation and feature map |
| [Frame index](frame-index.md) | Full-resolution evidence stills and whole-video contact sheets |
| [Transcript notes](transcript/README.md) | Transcription method, language, accuracy notes, and file formats |
| [Arabic transcript (VTT)](transcript/application-walkthrough-ar.vtt) | Timestamped WebVTT transcript |
| [Arabic transcript (SRT)](transcript/application-walkthrough-ar.srt) | Timestamped subtitle transcript |
| [Arabic transcript (plain text)](transcript/application-walkthrough-ar.txt) | Searchable original-language transcript |
| [Arabic transcript (JSON)](transcript/application-walkthrough-ar.json) | Machine-readable segments and metadata |

## Main conclusion

Remote is presented as a self-hosted, browser-first AI development and automation workspace. Each project is a separate Linux computer/container with durable files, its own chats, processes, ports, browser profile, tools, and secrets. The chat is the control plane for Codex, Claude, or Kimi; the surrounding UI exposes the same workspace through a browser IDE, terminal, file manager, Git history, live application preview, and a headed browser that the agent and user can share.

The distinctive product idea is not merely “chat with a coding model.” It is “give an interchangeable agent a persistent, isolated remote computer, then keep all development and browser-control surfaces in one web application.”

## Evidence and confidence

- The full recording was sampled every 10 seconds, producing 110 review frames across ten contact sheets.
- Twenty full-resolution key frames were extracted around feature transitions.
- The audio was transcribed with a multilingual Whisper medium model and voice-activity detection. The raw transcript is machine-generated and has not been human-corrected word by word.
- Feature claims are labeled in the analysis as **demonstrated**, **described**, or **inferred**. Narrator claims such as unlimited scale, strong isolation, and low resource use should be independently load- and security-tested.

