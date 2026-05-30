# Frontend to Backend API

This documents the backend HTTP and WebSocket surface used by the frontend in this repository.

Current source of truth:

- Top-level route registration: `backend/internal/transport/http/server.go`
- HTTP handlers: `backend/internal/transport/http/handlers/`
- WebSocket handlers: `backend/internal/transport/ws/`
- Frontend callers: `frontend/src/services/`, `frontend/src/hooks/`, `frontend/src/api/`

## Conventions

- Base URL: same origin as the loaded frontend, for example `https://remote.futrx.dev`.
- Auth: browser cookies. The frontend helper sends `credentials: "same-origin"`.
- Protected routes: `/api/*` and `/ws*` require the admin session cookie when Google auth is enabled.
- JSON request bodies use `Content-Type: application/json`.
- Upload endpoints use `multipart/form-data` with the field name `files`.
- Typical JSON error body: `{ "error": "message" }`.

## HTTP Endpoints Used By The Frontend

### Auth

| Method | Path | Frontend caller | Request | Response |
| --- | --- | --- | --- | --- |
| `GET` | `/auth/me` | `authService.getAuthSession()` | None | `AuthSession` JSON. The frontend treats `404` as open/no-auth mode. |
| `GET` | `/auth/google/login` | `LoginScreen` link | Optional `return_to` query. | Redirects to Google OAuth and sets a short-lived state cookie. |
| `GET` | `/auth/google/callback` | Google OAuth redirect | Query: `state`, `code`. | Sets session cookies and redirects to `/` or a validated `return_to`. |
| `GET` | `/auth/logout` | `AccountFooter` link | None | Clears session cookies and redirects to `/`. |

Registered but not called directly by frontend code:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/auth/verify` | Used by Caddy `forward_auth`. Returns `200` for an admin session or redirects to `/auth/google/login`. |

### Claude CLI Auth

| Method | Path | Frontend caller | Request | Response |
| --- | --- | --- | --- | --- |
| `GET` | `/api/claude/auth-status` | `claudeAuthService.status()` | None | `{ "authenticated": boolean }` based on server-side Claude credentials. |
| `POST` | `/api/claude/login/start` | `claudeAuthService.startLogin()` | `{}` | `{ "url": string, "resumed"?: true }`. Starts or resumes `claude auth login --claudeai`. |
| `POST` | `/api/claude/login/code` | `claudeAuthService.submitCode(code)` | `{ "code": string }` | `{ "success": true }` when the pasted OAuth code completes login. |
| `POST` | `/api/claude/login/cancel` | `claudeAuthService.cancelLogin()` | `{}` | `{ "ok": true }`. Cancels the active Claude login process. |

### Chats

| Method | Path | Frontend caller | Request | Response |
| --- | --- | --- | --- | --- |
| `GET` | `/api/chats` | `chatService.list()` | None | `ChatMeta[]`. Polled by workspace data. |
| `POST` | `/api/chats` | `chatService.create(body)` | `CreateChatInput` | `201` with `ChatMeta`. |
| `GET` | `/api/chats/{id}` | `chatService.get(id)` | None | `ChatMeta`. Loaded before opening the chat WebSocket. |
| `PATCH` | `/api/chats/{id}` | `chatService.update(id, body)` | `UpdateChatInput` | Updated `ChatMeta`. |
| `DELETE` | `/api/chats/{id}` | `chatService.delete(id)` | None | `{ "ok": true }`. |
| `GET` | `/api/chats/{id}/events` | `chatService.events(id)` | None | `ChatEvent[]`. Present in the service; the active chat screen primarily uses WebSocket replay. |
| `POST` | `/api/chats/{id}/rewind` | `chatService.rewind(id, beforeT)` | `{ "beforeT": number }` | `{ "events": ChatEvent[] }`. |
| `POST` | `/api/chats/{id}/upload` | `uploadChatFiles(chatId, files)` | Multipart field `files`, max 200 MiB total. | `{ "cwd": string, "results": UploadResult[] }`. |

### Projects

| Method | Path | Frontend caller | Request | Response |
| --- | --- | --- | --- | --- |
| `GET` | `/api/projects` | `projectService.list()` | None | `ProjectMeta[]`. Polled by workspace data. |
| `POST` | `/api/projects` | `projectService.create(name)` | `{ "name": string }` | `201` with `ProjectMeta`. |
| `GET` | `/api/projects/{id}` | `projectService.get(id)` | None | `ProjectMeta`. |
| `PATCH` | `/api/projects/{id}` | `projectService.update(id, body)` | `{ "name"?: string }` | Updated `ProjectMeta`. |
| `DELETE` | `/api/projects/{id}` | `projectService.delete(id)` | None | `{ "ok": true }`. |
| `POST` | `/api/projects/{id}/start` | `projectService.start(id)` | `{}` | Updated `ProjectMeta`. Starts the project container. |
| `POST` | `/api/projects/{id}/stop` | `projectService.stop(id)` | `{}` | Updated `ProjectMeta`. Stops the project container. |

## HTTP Endpoints Registered But Not Used By Current Frontend Code

### Tmux Sessions

| Method | Path | Request | Response or behavior |
| --- | --- | --- | --- |
| `GET` | `/api/sessions` | None | List tmux sessions. |
| `POST` | `/api/sessions` | `{ "name": string }` | `201` with `{ "name": string }`. |
| `DELETE` | `/api/sessions/{name}` | None | `{ "ok": true }`. Kills the tmux session. |
| `POST` | `/api/sessions/{name}/send` | `{ "text": string, "pressEnter"?: boolean }` | `{ "ok": true }`. Sends text to tmux. |
| `POST` | `/api/sessions/{name}/upload` | Multipart field `files`. | Uploads into the tmux session cwd. |

### Internal TLS Probe

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/internal/tls-ask?domain=...` | Loopback-only Caddy on-demand TLS allow check for project preview hosts. |

## WebSockets

### Chat Stream

`/ws/chat/{chatId}` is the primary WebSocket used by the chat screen.

Frontend entry points:

- URL builder: `frontend/src/api/websocket.ts`
- Hook: `frontend/src/hooks/chat/useChat.ts`

Connection details:

- Uses `wss://` when the frontend is served over HTTPS, otherwise `ws://`.
- Auth uses the same cookies as `/api/*`.
- The server rejects invalid chat ids with `400` and missing chats with `404`.
- On connection, the server replays persisted chat events, then sends a `sync` event with the current running state.

Client to server messages:

```json
{ "type": "prompt", "text": "User prompt text" }
```

Starts a Claude prompt run for the chat.

```json
{ "type": "cancel" }
```

Cancels the active Claude prompt run.

Server to client messages are JSON `ChatEvent` objects. Common types:

| Type | Meaning |
| --- | --- |
| `sync` | Current running state, for example `{ "type": "sync", "running": true }`. |
| `user` | Persisted user prompt. |
| `assistant_text` | Streamed assistant text delta. |
| `thinking` | Claude thinking text when present. |
| `tool_use_start` | Tool call started. Includes `id`, `name`, and `input`. |
| `tool_use_end` | Tool call completed. Includes `id`, `output`, and optional `isError`. |
| `system` | Progress/system event. Includes `subtype` and optional `data`. |
| `session` | Claude session id update. Includes `claudeSessionId`. |
| `complete` | Claude run completed. Includes raw Claude `usage` when available. |
| `error` | User-visible error. Includes `message`. |

### Tmux Terminal Stream

`/ws?session={name}` is registered for terminal/tmux streaming. Current frontend code does not call it, but browser clients can use it.

Connection details:

- Query parameter `session` must be a valid tmux session name.
- If the session does not exist, the backend creates it before attaching.
- Server to client messages are binary PTY output bytes.

Client to server messages:

- Binary WebSocket messages are written directly to the PTY.
- Text JSON control messages:

```json
{ "type": "input", "data": "raw terminal input" }
```

```json
{ "type": "resize", "cols": 120, "rows": 40 }
```

## Main Data Shapes

### AuthSession

```ts
interface AuthSession {
  noAuth: boolean;
  authenticated: boolean;
  claimed: boolean;
  adminEmail: string;
  email: string;
  isAdmin: boolean;
}
```

### ChatMeta

```ts
interface ChatMeta {
  id: string;
  title: string;
  claudeSessionId?: string;
  tmuxSession?: string;
  cwd?: string;
  createdAt: number;
  lastMessageAt: number;
  model?: string;
  mode?: "chat" | "plan" | "code" | "review" | "debug" | "full-auto";
  projectId?: string;
}
```

### CreateChatInput

```ts
interface CreateChatInput {
  tmuxSession?: string;
  cwd?: string;
  title?: string;
  model?: string;
  mode?: string;
  projectId?: string;
}
```

### UpdateChatInput

```ts
interface UpdateChatInput {
  title?: string;
  cwd?: string;
  model?: string;
  mode?: string;
}
```

### ProjectMeta

```ts
type ProjectStatus = "" | "provisioning" | "running" | "stopped" | "error" | "missing";

interface ProjectMeta {
  id: string;
  name: string;
  slug: string;
  cwd: string;
  containerName: string;
  status: ProjectStatus;
  errorMsg?: string;
  createdAt: number;
  updatedAt: number;
}
```

### Upload Response

```ts
interface UploadResult {
  name: string;
  path?: string;
  size?: number;
  error?: string;
}

interface UploadChatFilesResponse {
  cwd: string;
  results: UploadResult[];
}
```
