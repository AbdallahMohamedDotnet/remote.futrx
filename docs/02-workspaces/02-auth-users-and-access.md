# Authentication, users, and access

The application separates three concerns:

1. Platform identity: who may open `remote.futrx`.
2. Agent credentials: whether a provider can run, with host-wide onboarding
   for Claude, Codex, and Kimi and project-local sign-in for Antigravity.
3. Project membership: which registered users may access a project.

## Application gate

```mermaid
stateDiagram-v2
    [*] --> CheckServerClaim
    CheckServerClaim --> ClaimAdmin: server is unclaimed
    CheckServerClaim --> SignIn: server is claimed
    ClaimAdmin --> CheckLocalAdmin: admin creates email and password
    SignIn --> CheckTwoFactor: valid password or Google credentials
    CheckTwoFactor --> TwoFactorChallenge: account has 2FA enabled
    CheckTwoFactor --> CheckLocalAdmin: account has no 2FA record
    TwoFactorChallenge --> CheckLocalAdmin: correct authenticator code or recovery code
    CheckLocalAdmin --> WaitForAdmin: legacy admin password setup is incomplete
    CheckLocalAdmin --> CheckProvider: local admin is configured
    CheckProvider --> ConnectProvider: no agent provider is authenticated and caller is admin
    CheckProvider --> WaitForProvider: no provider is authenticated and caller is member
    ConnectProvider --> Workspace: any provider becomes authenticated
    WaitForProvider --> Workspace: admin connects a provider
    CheckProvider --> Workspace: provider already authenticated
```

The backend middleware applies the same order to `/api/*` and `/ws*`: valid registered session, completed local-admin setup, then at least one authenticated agent provider. Provider-login endpoints are exempt from the final check so onboarding can finish. The `CheckTwoFactor`/`TwoFactorChallenge` step only exists for accounts that have opted into TOTP 2FA (see below); every other account skips straight from `SignIn` to `CheckLocalAdmin`, unchanged from before 2FA existed.

## Identities and roles

| Identity | Sign-in | Scope |
| --- | --- | --- |
| Local administrator | Email and a password of 12–1024 characters | Cannot be removed or demoted; manages host-wide setup |
| Invited administrator | Google OAuth | Admin routes and all projects |
| Invited member | Google OAuth | Only projects where their email is a member, plus loose chats |

Sessions are signed in the `remote_session` secure, HTTP-only cookie and last 30 days. Passwords use a salted, one-way hash. Google OAuth is optional until the admin wants to invite users.

## First administrator flow

```mermaid
sequenceDiagram
    actor Admin
    participant UI
    participant Auth as Auth service
    participant Store as File auth store
    participant Users as User directory

    Admin->>UI: Submit email and password
    UI->>Auth: POST /auth/local/claim
    Auth->>Store: Confirm server is unclaimed
    Auth->>Store: Save password hash
    Auth->>Users: Add bootstrap admin
    Auth->>Store: Sign session cookie
    Store-->>UI: Authenticated status
```

The first claim is public only while no admin exists. A legacy installation that already has an administrator identity but no password requires authorization by that existing admin.

## Two-factor authentication

From Settings → Security, any account (local admin or invited user) can independently turn on:

| Toggle | Effect | Depends on |
| --- | --- | --- |
| **Two-factor authentication (TOTP)** | Password/Google login returns a pending challenge instead of a session; `POST /auth/2fa/verify` (a 6-digit authenticator code, or an unused recovery code) completes it | Nothing |
| **Single active session** | A new login immediately supersedes the account's previous session | Nothing — works with or without 2FA |
| **Sign-in history** | Every login (bounded, newest-first) is recorded and shown in the Security tab | Nothing — works with or without 2FA |
| **Recovery-code alert** | A login completed with a recovery code (instead of a normal authenticator code) sets an alert shown on `/auth/me` until acknowledged | Two-factor authentication must already be on — recovery codes only exist once 2FA is enrolled |

Enrollment issues a TOTP secret (shown as a QR code and as text for manual entry), confirms one code from the user's authenticator app, and returns ten one-time recovery codes shown exactly once. Confirming enrollment, or turning on single active session, re-issues the browser's *current* session as tracked in the same response, so the tab that just made the change is never immediately treated as "the other device."

An account that leaves all four toggles off — the default for every account, including ones that predate this feature — sees byte-for-byte the same login flow, session cookie, and `/auth/me` response as before any of this existed.

## Invited user flow

```mermaid
flowchart LR
    OAuth["Admin saves Google client ID and secret"] --> Invite["Admin registers user email and role"]
    Invite --> Login["User chooses Google sign-in"]
    Login --> Callback["Google callback returns verified identity"]
    Callback --> Registered{"Email is registered?"}
    Registered -->|"No"| Deny["Deny access"]
    Registered -->|"Yes"| Session["Issue platform session"]
    Session --> Projects["Show permitted projects and chats"]
```

User guardrails:

- Google sign-in must be configured before users can be added.
- Only admins can add users, remove users, or change roles.
- The last administrator cannot be removed or demoted.
- The local administrator cannot be removed or demoted.

## Agent-provider authentication

Claude, Codex, and Kimi credentials are host-wide and admin-managed.

```mermaid
flowchart TD
    Start["Admin opens Agent settings"] --> Provider{"Provider"}
    Provider -->|"Claude"| ClaudeStart["Start CLI authorization-code flow"]
    ClaudeStart --> ClaudeURL["Open authorization URL"]
    ClaudeURL --> ClaudeCode["Paste returned code"]
    ClaudeCode --> Saved["Credentials detected on host"]

    Provider -->|"Codex"| CodexDevice["Start device-code login"]
    Provider -->|"Kimi"| KimiDevice["Start device-code login"]
    CodexDevice --> Verify["Open verification URL and enter code"]
    KimiDevice --> Verify
    Verify --> Saved
    Saved --> Broadcast["Status WebSocket updates onboarding UI"]
```

Claude uses an interactive authorization URL plus a pasted code. Codex and Kimi use device-code flows. Credential files are later synchronized into project containers before agent execution.

Antigravity is deliberately outside this host-wide flow. Its bare `agy` sign-in
never exits the interactive TUI, so Remote exposes no Antigravity card or
onboarding status. A user runs `agy` once in a project Terminal and completes
the URL-and-code flow there. That project-local state does not satisfy the
application's initial provider gate and is not synchronized by the host
credential service.

## Project access rules

```mermaid
flowchart TD
    Request["Authenticated request for a project resource"] --> Admin{"Caller is admin?"}
    Admin -->|"Yes"| Allow["Allow"]
    Admin -->|"No"| Member{"Email is in project access list?"}
    Member -->|"Yes"| Allow
    Member -->|"No"| Deny["403 Forbidden"]
```

| Operation | Admin | Project member |
| --- | ---: | ---: |
| See project and its chats | Yes | Yes |
| Create chat in project | Yes | Yes |
| Start, stop, restart, inspect, repair | Yes | Yes |
| Read or change secrets | Yes | Yes |
| Edit project membership | Yes | Yes, but cannot remove the final member |
| Set resource limits | Yes | No |
| Delete project | Yes | No |
| Manage global users or Google OAuth | Yes | No |
| Connect agent providers | Yes | No |

Project access is enforced independently on project/chat HTTP resources, chat sockets, terminal sockets, uploads, workspace snapshots, project skills, and preview requests.

## Preview and IDE authentication

Caddy calls `/auth/verify` before forwarding IDE or preview traffic. For a preview host, the backend extracts the project slug and checks membership. IDE hosts currently receive the registered-user check but not a per-project membership check. After verification, Caddy strips platform session cookies before the request enters project-controlled code.

```mermaid
sequenceDiagram
    actor User
    participant Caddy
    participant Auth as /auth/verify
    participant Project as Project app or IDE

    User->>Caddy: Request project subdomain
    Caddy->>Auth: Forward-auth with session and original host
    Auth->>Auth: Validate session, and preview project membership
    Auth-->>Caddy: 200, redirect, or deny
    Caddy->>Project: Proxy without platform auth cookies
    Project-->>User: App or IDE response
```

## Code map

- Frontend gate: [`frontend/src/app/containers/AuthGate.tsx`](../../frontend/src/app/containers/AuthGate.tsx)
- Auth middleware: [`backend/internal/transport/http/middleware/auth.go`](../../backend/internal/transport/http/middleware/auth.go)
- Auth service: [`backend/internal/service/auth/service.go`](../../backend/internal/service/auth/service.go)
- TOTP/recovery-code sub-component: [`backend/internal/service/auth/twofactor.go`](../../backend/internal/service/auth/twofactor.go)
- Session registry (single-session, history, alert): [`backend/internal/service/auth/session_registry.go`](../../backend/internal/service/auth/session_registry.go)
- 2FA challenge and Security-tab handlers: [`backend/internal/transport/http/handlers/auth_twofactor_handler.go`](../../backend/internal/transport/http/handlers/auth_twofactor_handler.go), [`backend/internal/transport/http/handlers/security_handler.go`](../../backend/internal/transport/http/handlers/security_handler.go)
- User service: [`backend/internal/service/user/service.go`](../../backend/internal/service/user/service.go)
- Access adapter: [`backend/internal/transport/transport.go`](../../backend/internal/transport/transport.go)
