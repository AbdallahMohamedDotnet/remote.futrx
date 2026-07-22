# Previews and browser features

There are two browser systems:

| System | Purpose | Browser process |
| --- | --- | --- |
| App preview | Show a web app already running in the project | The user's normal browser loads the project app in an iframe |
| Agent Browser | Share a signed-in, headed browser between user and agent | Chromium runs inside the project container |

## App discovery and preview URLs

```mermaid
flowchart LR
    App["Project process listens on TCP port"] --> Scan["Backend scans with ss"]
    Scan --> Filter["Exclude loopback-only listeners and deduplicate ports"]
    Filter --> Picker["Browser drawer app picker"]
    Picker --> URL["https://slug--port.dev.host"]
    URL --> Caddy["Caddy authenticates request"]
    Caddy --> DNS["slug.lxd:port"]
    DNS --> App
```

The UI prefers a preview URL recently mentioned in chat when it matches the current project and a discovered port. Otherwise it selects a discovered listener.

Preview host rules:

- Port must be between 1024 and 65535.
- On-demand TLS asks the backend to confirm the slug is a real project before certificate issuance.
- The authenticated user must be an admin or project member.
- Platform cookies are stripped before the request enters project code.

## Preview inspection

Inspect mode wraps the selected app with `/__remote_inspector` on the same preview origin. Same-origin access lets the wrapper inspect the inner app iframe.

```mermaid
sequenceDiagram
    actor User
    participant UI as Main chat UI
    participant Wrapper as Inspector wrapper
    participant App as Project app iframe

    User->>UI: Enable inspect mode
    UI->>Wrapper: Load wrapper with app URL
    Wrapper->>App: Load app on the same preview origin
    Wrapper-->>UI: remote-inspector:ready
    UI->>Wrapper: Enable selection
    User->>App: Hover and click element
    Wrapper->>Wrapper: Collect selector, text, HTML, box, styles, parents
    Wrapper-->>UI: element-selected payload
    UI->>UI: Insert Browser element block into composer
```

The payload is bounded and includes enough layout and accessibility context for an agent to identify the selected element. Selecting an element exits inspect mode.

## Agent Browser architecture

```mermaid
flowchart LR
    User["User"] -->|"noVNC iframe"| View["VNC and noVNC view"]
    Agent["Selected agent CLI"] -->|"MCP over CDP"| Chrome["Headed Chromium"]
    View --> Display["Shared virtual display"]
    Chrome --> Display
    Chrome --> Profile["Persistent browser profile in workspace"]
    Display --> Xvfb["Xvfb and window manager"]
```

The user can sign in visually while the agent controls the same Chromium session. Browser profile data lives under the project workspace, so site logins survive container replacement.

## Agent Browser lifecycle

```mermaid
stateDiagram-v2
    [*] --> Stopped
    Stopped --> Starting: open Agent Browser or select browser skill
    Starting --> CoreReady: Chromium and CDP ready, no human view
    Starting --> Ready: core and noVNC ready
    CoreReady --> Ready: start human view
    Ready --> CoreReady: close drawer, stop view only
    CoreReady --> Stopped: explicit stop or idle reaper
    Ready --> Stopped: explicit stop or idle reaper
    Starting --> Error: provision or start failure
    Error --> Starting: retry
```

Frontend behavior:

- Opening Agent Browser calls the start endpoint, polls every 1.5 seconds, and sends a status heartbeat every 15 seconds.
- Closing the drawer stops only the noVNC view; the agent-facing core stays available.
- Explicit Stop tears down the complete browser stack but keeps the profile.

Backend behavior:

- Starting the browser first ensures the project container is running.
- Provisioning installs missing browser packages and republishes versioned scripts.
- A selected `browser` skill enables the provider's browser MCP config.
- Active browser-enabled prompts send a keepalive every minute.
- A reaper checks every minute and stops a browser after 20 minutes without pane or agent activity, unless a viewer is connected.

## URL and proxy layout

```mermaid
flowchart TD
    Main["https://host"] --> Backend["Main UI and API"]
    IDELauncher["https://code.host"] --> Launcher["Installable IDE launcher"]
    IDEProject["https://slug.code.host"] --> CodeServer["slug.lxd:8842"]
    Preview["https://slug--port.dev.host"] --> ProjectApp["slug.lxd:port"]
    AgentView["https://slug--6080.dev.host"] --> NoVNC["slug.lxd:6080"]
```

The installer configures host DNS resolution for `.lxd` names through the LXD bridge. Caddy handles public HTTPS and routes to private container addresses.

## Security boundary

```mermaid
flowchart LR
    Request["Public subdomain request"] --> TLS["On-demand TLS allow check"]
    TLS --> Auth["Platform session and membership check"]
    Auth --> Strip["Strip platform cookies"]
    Strip --> Container["Untrusted project app, IDE, or noVNC"]
```

Project apps may set and receive their own cookies. Only the platform's session, OAuth-state, and return-location cookies are removed.

## Code map

- App scanning: [`backend/internal/integration/containers/listeners/scanner.go`](../../backend/internal/integration/containers/listeners/scanner.go)
- Browser drawer: [`frontend/src/ui/chat/browser/BrowserDrawer.tsx`](../../frontend/src/ui/chat/browser/BrowserDrawer.tsx)
- Inspector handler: [`backend/internal/transport/http/handlers/browser_inspector_handler.go`](../../backend/internal/transport/http/handlers/browser_inspector_handler.go)
- Agent Browser service: [`backend/internal/service/container/browser/service.go`](../../backend/internal/service/container/browser/service.go)
- Caddy routes: [`infra/templates/Caddyfile.tmpl`](../../infra/templates/Caddyfile.tmpl)
