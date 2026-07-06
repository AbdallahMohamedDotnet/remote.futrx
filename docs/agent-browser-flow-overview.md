# Agent Browser Flow: Overview

Companion pages:

- [Human view flow](agent-browser-flow-human-view.md)
- [Agent run flow](agent-browser-flow-agent-run.md)
- [Lifecycle flow](agent-browser-flow-lifecycle.md)
- [Main Agent Browser doc](agent-browser.md)

The Agent Browser has two access paths into one shared Chrome session:

- The human path exposes pixels through noVNC on port `6080`.
- The agent path drives Chrome through CDP on `127.0.0.1:9222`.

```mermaid
flowchart LR
    UI["Chat UI"] -->|"open pane"| View["Human view<br/>noVNC :6080"]
    Prompt["Prompt with browser skill"] --> Provider["Claude / Codex provider"]
    Provider -->|"start core + MCP"| MCP["@playwright/mcp<br/>--caps=vision"]
    MCP -->|"loopback CDP"| CDP["127.0.0.1:9222"]

    View --> Chrome["One headed Chrome<br/>same profile"]
    CDP --> Chrome
    Chrome <--> Profile["/workspace/.browser-gui/profile"]
    Chrome --> Sites["Target sites"]

    classDef shared fill:#7ba7ff,stroke:#0b0d11,color:#fff;
    classDef persisted fill:#b7e4c7,stroke:#1b6b3a,color:#111;
    class Chrome shared;
    class Profile persisted;
```

Key invariant: the human and the agent do not get separate browsers. They both
attach to the same headed Chrome, so the agent can reuse cookies from the
login the user performed by hand.
