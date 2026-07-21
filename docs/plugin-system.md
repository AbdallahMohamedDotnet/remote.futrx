# Plugin System: Design

> Status: **implemented** (design and initial Terminal plugin landed 2026-07-21).
> Companion API reference: [frontend-backend-api.md](frontend-backend-api.md).

## The capability we are building

**Features become installable modules.** A plugin bundles everything one
feature needs — backend routes/services and frontend UI — behind a single ID.
An admin can install or uninstall it at runtime from a dashboard in Settings:
uninstalled means its endpoints return 404, its UI never renders, and its
frontend code is never even downloaded. The container terminal ("Open
Terminal") is the first feature planned to move into a plugin.

## Locked decisions

1. **Compiled-in catalog.** All plugins ship inside the Go binary and the
   Vite bundle (as lazy chunks). Install/uninstall toggles *activation state*
   at runtime — it does not download or delete code. Rationale:
   - The app deploys as one binary with the frontend embedded
     (`static.go` `//go:embed public`); there is no artifact channel to
     fetch plugin code from, and Go's `plugin` package is too fragile for
     dynamic loading.
   - The app wields `lxc exec` into project containers; loading third-party
     code into this process would be a serious security liability. The
     catalog is first-party code only.
   - The interfaces below are deliberately data-driven (manifest structs,
     ID-keyed registry, request-time gating) so a *truly external* plugin
     host — e.g. out-of-process plugins, even per-plugin LXD containers
     reverse-proxied under `/plugin/{id}/` — could join the same registry
     later without reworking the core. That is explicitly **out of scope**
     now.
2. **v1 ships the framework and one real plugin.** Registry, state store,
   admin API, frontend slots, the Plugins settings panel, and the Terminal
   plugin land together so the public seams are exercised end to end.
3. **The dashboard lives in Settings and is admin-only**, as a section card
   following the existing admin-gated sections pattern.

## Semantics of install / uninstall

- **Instance-wide, admin-managed.** One state for the whole deployment, not
  per-user. (Per-user visibility could later be a plugin's own setting.)
- **Runtime toggle, no restart.** Routes are registered once at boot; a
  request-time gate consults in-memory state.
- **Uninstalled = indistinguishable from nonexistent.** Gated routes return
  `404` (matching the existing "nil service → feature absent" idiom), slot
  hosts skip the plugin, and its lazy chunk is never fetched.
- **Defaults.** Each manifest declares `InstalledByDefault`. A plugin ID with
  no entry in the state file uses its default; the file stores only explicit
  overrides, so deleting `data/plugins.json` resets to defaults. Terminal
  ships with `InstalledByDefault: true` — upgrading changes nothing until an
  admin uninstalls.
- **Live resources.** Uninstall blocks *new* requests immediately; plugins
  holding live resources (e.g. terminal PTY websockets) implement an
  optional `Deactivate(ctx)` hook, called on uninstall, to close them.

## Current source of truth (the seams this builds on)

- Route composition + the `RouteRegistrar` / `WebSocketRegistrar` interfaces:
  `backend/internal/transport/http/server.go`
- Edge composition root (handler construction, `accessGate`):
  `backend/internal/transport/transport.go`
- Domain composition root: `backend/internal/service/services.go`
- Store assembly: `backend/internal/stores/stores.go`; file-store idiom
  (atomic temp-file + rename, mutex): `backend/internal/stores/fileusersettings/store.go`
- Admin gating idiom (`requireAdmin`, `callerStateFromRequest`):
  `backend/internal/transport/http/handlers/users_handler.go`
- Terminal backend plugin:
  `backend/internal/plugin/terminal/plugin.go`
- Frontend lazy-load seams: `frontend/src/context/PluginsContext.tsx` and
  `frontend/src/plugins/ChatPluginHost.tsx`
- Hardcoded surfaces the slots replace:
  chat header action bar `frontend/src/components/chat/header/CwdEditor.tsx`,
  overlay mounts `frontend/src/containers/ChatContainer.tsx`,
  settings stack `frontend/src/components/settings/SettingsPage.tsx`

## Backend design

### `internal/plugin` — SPI types and registry

One new package holding the plugin SPI and the registry (analogous to the
cross-cutting `internal/agent` package):

```go
// internal/plugin/plugin.go
type Manifest struct {
    ID                 string // stable kebab-case, e.g. "terminal"
    Name               string
    Description        string
    Version            string
    InstalledByDefault bool
}

// A route is declared as data, not registered by the plugin itself, so the
// registry can wrap every handler in the install gate — a plugin cannot
// accidentally expose an ungated route.
type Route struct {
    Pattern string       // http.ServeMux pattern, e.g. "/ws/terminal"
    Handler http.Handler
}

type Plugin interface {
    Manifest() Manifest
    Routes(upgrader websocket.Upgrader) []Route
}

// Optional; called when the plugin is uninstalled at runtime.
type Deactivatable interface {
    Deactivate(ctx context.Context) error
}

// Optional; closes the race between the route gate and live resource setup.
type InstallationListener interface {
    PluginInstallationChanged(installed bool)
}
```

```go
// internal/plugin/ports.go
type Repository interface {
    All(ctx context.Context) (map[string]State, error)          // explicit overrides only
    Save(ctx context.Context, id string, st State) (State, error)
}
type State struct{ Installed bool }
```

```go
// internal/plugin/registry.go
func NewRegistry(ctx context.Context, repo Repository, plugins ...Plugin) (*Registry, error)

func (r *Registry) Register(plugin Plugin) error
func (r *Registry) Catalog() []Entry                // manifest + installed, for the admin API
func (r *Registry) InstalledIDs() []string          // for GET /api/plugins
func (r *Registry) IsInstalled(id string) bool      // in-memory, RWMutex-guarded
func (r *Registry) SetInstalled(ctx context.Context, id string, installed bool) (Entry, error)
func (r *Registry) Mount(mux *http.ServeMux, up websocket.Upgrader)
```

- `NewRegistry` loads overrides once, merges with manifest defaults, and
  rejects duplicate IDs.
- `Mount` registers every plugin route wrapped in the gate:
  uninstalled → `http.NotFound`. State checks never touch disk.
- Duplicate plugin IDs and route patterns are rejected during startup.
- `SetInstalled` persists via the repository, updates the in-memory state,
  and on `true → false` calls `Deactivate` when implemented.

### State store

`internal/stores/fileplugins` following the house file-store idiom
(compile-time `var _ plugin.Repository = (*Store)(nil)`, atomic write,
mutex). Single file `<dataDir>/plugins.json`:

```json
{ "plugins": { "terminal": { "installed": false } } }
```

Reserved for later: a per-plugin `"config": {}` object next to `installed`.

### API

| Method | Path | Access | Response |
| --- | --- | --- | --- |
| `GET` | `/api/plugins` | any registered user | `{ "installed": ["terminal"] }` — the frontend boots from this |
| `GET` | `/api/admin/plugins` | admin | `{ "plugins": [{ "id", "name", "description", "version", "installed", "installedByDefault" }] }` |
| `POST` | `/api/admin/plugins/{id}/install` | admin | updated entry |
| `POST` | `/api/admin/plugins/{id}/uninstall` | admin | updated entry |

Admin gating uses `callerStateFromRequest` → 403 `"admin only"`. With auth
disabled (`nil` auth service), the handler explicitly treats the solo user as
the local administrator. Unknown plugin ID → 404; toggling to the current
state is a no-op success (idempotent).

### Wiring changes

- `internal/stores/stores.go`: construct `fileplugins.Store`.
- `internal/service/services.go`: construct `plugin.Registry` and expose it on
  `Services`.
- `internal/transport/transport.go`: register the Terminal plugin with narrow
  chat/project/access dependencies, construct `PluginsHandler`, and add both
  the admin handler and plugin route mounter to `Handlers`.
- `internal/transport/http/server.go`: `NewHandler` additionally calls
  `registry.Mount(mux, upgrader)` (accepted as a small `PluginMounter`
  interface, keeping the transport decoupled from the concrete registry).

## Frontend design

### Catalog and types

```
src/plugins/
  types.ts                // PluginModule + slot types (below)
  catalog.ts              // ID + lazy module loader only
  ChatPluginHost.tsx      // header and overlay slot hosts
  PluginSettingsSections.tsx
  terminal/               // first real plugin
```

```ts
// src/plugins/types.ts
import type { ComponentType } from "preact";

export interface ChatOverlayProps {
  chat: ChatMeta;
  open: boolean;
  onClose: () => void;
}

export interface PluginModule {
  id: string; // must match the backend manifest ID
  slots: {
    chatHeaderAction?: { label: string; Icon: ComponentType<{ class?: string }> };
    chatOverlay?: () => Promise<{ default: ComponentType<ChatOverlayProps> }>;
    settingsSection?: () => Promise<{ default: ComponentType }>;
  };
}

export interface PluginCatalogEntry {
  id: string;
  load: () => Promise<{ default: PluginModule }>;
}
```

The catalog itself contains only dynamic `import()` loaders. `PluginsContext`
checks the installed-ID response before invoking a loader, so Vite emits one
descriptor chunk per plugin and an uninstalled plugin costs zero bytes on the
wire. Heavy overlays such as xterm remain a second lazy chunk loaded on first
open.

### Installed-state context

- `src/services/pluginService.ts` — house service-object style over
  `api/http.ts`: `installed()`, `adminList()`, `install(id)`, `uninstall(id)`.
- `src/context/PluginsContext.tsx` — mounted in `AppProviders` inside
  `UserSettingsProvider`; fetches `GET /api/plugins` once Google/solo auth is
  available; exposes `{ installed, modules, loading, error, refresh }`.

### Slot hosts (replacing hardcoded surfaces)

1. **Chat header actions** — `CwdEditor.tsx` renders installed plugins'
   `chatHeaderAction` buttons after the built-in ones; clicking dispatches
   "open overlay for plugin X".
2. **Chat overlays** — `ChatPluginHostProvider` owns plugin open/activated
   state, resets open drawers on chat switch, lazy-loads each overlay on first
   open, and renders it through `ChatPluginOverlays` in `ChatContainer`.
3. **Settings sections** — `SettingsPage` renders installed plugins'
   `settingsSection` components at the end of the section stack. Plugin
   sections must be self-contained (fetch their own data via their service),
   not add to `SettingsPage`'s prop list.

All three hosts safely render nothing when no installed plugin contributes to
their slot. Terminal exercises the header and overlay slots in production.

## The Plugins dashboard

`frontend/src/components/settings/PluginsPanel.tsx`, a section card in the
existing style (icon tile + title + subtitle header), rendered in
`SettingsPage` wrapped in `{isAdmin && …}` exactly like the Claude/Codex
auth sections. Self-contained: loads `pluginService.adminList()`, renders
name / description / version / Install-or-Uninstall button per plugin, and
shows an empty state when a future build has no registered plugins.
After a toggle it refreshes both its own list and `PluginsContext`, so the
admin's UI reflects the change immediately.

The sidebar shows Settings whenever the account is authenticated, including
solo/no-auth mode. The solo user is already treated as admin (`authService`
maps 404 → `isAdmin: true`).

## Anatomy of one plugin (Terminal)

One ID — `"terminal"` — connects:

- **`backend/internal/plugin/terminal/`** — the moved
  `container_terminal_socket.go` logic; manifest
  `{ID: "terminal", InstalledByDefault: true}`; one declared route
  `{Pattern: "/ws/terminal", Handler: …}` (path unchanged — no client
  break); tracks live PTY connections and closes them in `Deactivate`.
- **`frontend/src/plugins/terminal/`** — the moved button descriptor
  (label "Open Terminal", `Terminal` icon), `chatOverlay` lazy-importing
  `TerminalOverlay`, plus the moved session hook. Kills today's
  `onOpenTerminal` prop-drill through `CwdEditor ← ThreadHeader ←
  ChatThread ← ChatContainer`.
- One backend registration in `transport.go` and one lazy frontend catalog
  entry in `catalog.ts`.

## Phases and acceptance criteria

### Phase 1 — Framework (implemented)

- [x] **F1.** `internal/plugin` package: `Manifest`, `Route`, `Plugin`,
  `Deactivatable`, `Registry` with cached state, defaults merge, duplicate-ID
  rejection.
- [x] **F2.** `fileplugins` store at `data/plugins.json`; missing file or
  entry → manifest default; atomic writes.
- [x] **F3.** `Mount` wraps every plugin route in the gate; uninstalled →
  404; install/uninstall takes effect without restart.
- [x] **F4.** The four endpoints above, admin-gated per the
  `users_handler.go` pattern; idempotent toggles; unknown ID → 404.
- [x] **F5.** Registry unit tests using a fake plugin: default state, toggle
  persistence, gate 404, `Deactivate` invoked on uninstall.
- [x] **F6.** `pluginService` + `PluginsContext` (installed set fetched after
  the auth gate opens).
- [x] **F7.** Three slot hosts wired (header actions, chat overlays,
  settings sections); empty slots render nothing and Terminal exercises the
  chat slots.
- [x] **F8.** `PluginsPanel` in Settings, admin-only, with install/uninstall
  and an empty state.
- [x] **F9.** Settings reachable in solo/no-auth mode.
- [x] **F10.** [frontend-backend-api.md](frontend-backend-api.md) updated
  with the new endpoints.

### Phase 2 — Terminal becomes the first plugin (implemented with Phase 1)

- [x] **T1.** Backend socket moved to `internal/plugin/terminal`; `/ws/terminal`
  path and message protocol unchanged; old registration removed.
- [x] **T2.** `InstalledByDefault: true` — upgrade is behavior-neutral.
- [x] **T3.** Frontend terminal module under `src/plugins/terminal/`;
  hardcoded button and `onOpenTerminal` prop-drill removed.
- [x] **T4.** Uninstall: button disappears, new WS connections get 404, live
  PTY sessions are closed by `Deactivate`, terminal chunk is not fetched by
  a fresh page load.
- [x] **T5.** Reinstall restores the feature without a page reload for the
  admin.

### Phase 3 — Candidates (not yet committed)

Open Browser, History, and Files share the terminal's action + drawer/overlay
shape; Open in IDE is an action-only candidate. The tmux bridge is a special
case: decide whether to delete the dead `internal/service/tmux` package
(imported by nothing today) or resurrect it as the tmux plugin.

## Open items / later enhancements

- **Live propagation to other users:** broadcast plugin-state changes over
  the existing `workspacehub` so already-connected non-admin clients update
  without a reload. Optional; a reload is acceptable initially.
- **Per-plugin config:** the state file and manifest leave room for a
  `config` object + a plugin-provided settings section; no consumer yet.
- **External plugins / per-user installs / marketplace:** explicitly out of
  scope (see locked decision 1).
