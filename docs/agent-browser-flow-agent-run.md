# Agent Browser Flow: Agent Run

Related flows:

- [Overview flow](agent-browser-flow-overview.md)
- [Human view flow](agent-browser-flow-human-view.md)
- [Lifecycle flow](agent-browser-flow-lifecycle.md)

This is the path used when a prompt runs with the `browser` skill selected.

```mermaid
flowchart TB
    Prompt["User prompt<br/>browser skill selected"] --> PromptSvc["Prompt service"]
    PromptSvc -->|"EnableBrowser"| Provider["Claude / Codex provider"]
    Provider -->|"EnsureAgentBrowserMCP"| MCPConfig["Browser MCP config"]
    Provider -->|"EnsureAgentBrowserCore"| Core["Xvfb + openbox + Chrome"]

    Agent["Claude / Codex process"] --> MCP["@playwright/mcp<br/>browser_* tools"]
    MCPConfig --> MCP
    MCP -->|"CDP on 127.0.0.1:9222"| Core
    Core <--> Profile["Persistent profile"]
    Core --> Sites["Target sites"]

    PromptSvc -->|"activity heartbeat"| ProjectSvc["Project service"]

    classDef private fill:#a7d8ff,stroke:#15527a,color:#111;
    classDef persisted fill:#b7e4c7,stroke:#1b6b3a,color:#111;
    class Core,MCP,MCPConfig private;
    class Profile persisted;
```

The provider starts the core browser stack automatically, so the agent does not
need the user to open the pane just to start browsing. CDP remains loopback-only
inside the project container.
