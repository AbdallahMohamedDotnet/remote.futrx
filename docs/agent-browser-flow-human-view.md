# Agent Browser Flow: Human View

Related flows:

- [Overview flow](agent-browser-flow-overview.md)
- [Agent run flow](agent-browser-flow-agent-run.md)
- [Lifecycle flow](agent-browser-flow-lifecycle.md)

This is the path used when the user opens the Browser pane to watch the
session or log in by hand.

```mermaid
flowchart TB
    UI["Chat UI<br/>Browser pane"] -->|"POST /api/projects/{id}/agent-browser/start"| Backend["Go backend"]
    Backend --> ProjectSvc["Project service"]
    ProjectSvc -->|"EnsureAgentBrowserView"| Manager["Container manager"]
    Manager -->|"gui-up.sh start-view"| ViewStack["x11vnc + websockify<br/>noVNC :6080"]
    ViewStack --> Chrome["Shared headed Chrome"]

    ViewStack -->|"&lt;slug&gt;--6080.dev.&lt;host&gt;"| Caddy["Caddy edge<br/>Google auth"]
    Caddy --> UI

    UI -->|"DELETE /agent-browser?scope=view<br/>on drawer close"| Backend
    ProjectSvc -->|"StopAgentBrowserView"| ViewStack

    classDef exposed fill:#f7d774,stroke:#8a6d00,color:#111;
    classDef shared fill:#7ba7ff,stroke:#0b0d11,color:#fff;
    class ViewStack exposed;
    class Chrome shared;
```

Only noVNC port `6080` is exposed, and it is exposed through the auth-gated
dev-URL proxy. Closing the drawer stops only x11vnc/websockify; the Chrome
core can keep running for the agent.
