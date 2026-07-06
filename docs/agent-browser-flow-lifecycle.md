# Agent Browser Flow: Lifecycle

Related flows:

- [Overview flow](agent-browser-flow-overview.md)
- [Human view flow](agent-browser-flow-human-view.md)
- [Agent run flow](agent-browser-flow-agent-run.md)

This flow explains when the stack starts, what stops, and how idle cleanup
works.

```mermaid
flowchart TB
    Start["Browser requested"] --> Core{"Core ready?"}
    Core -- "no" --> StartCore["start-core<br/>Xvfb + openbox + Chrome"]
    Core -- "yes" --> NeedView{"Human view needed?"}
    StartCore --> NeedView

    NeedView -- "yes" --> StartView["start-view<br/>x11vnc + websockify"]
    NeedView -- "no" --> AgentOnly["Agent-only session"]
    StartView --> Watched["Human watching / login"]

    Watched -->|"drawer close"| StopView["stop-view only"]
    StopView --> AgentOnly

    AgentOnly --> Heartbeat["agent and status heartbeats"]
    Watched --> Heartbeat
    Heartbeat --> Reaper{"Idle past TTL<br/>and no viewers?"}
    Reaper -- "no" --> Heartbeat
    Reaper -- "yes" --> StopAll["stop full stack"]
    StopAll --> Stopped["stopped<br/>profile remains on disk"]
    Stopped --> Start
```

The profile is not deleted when the stack stops. On the next start, Chrome
comes back with the same profile and the same user-performed logins.
