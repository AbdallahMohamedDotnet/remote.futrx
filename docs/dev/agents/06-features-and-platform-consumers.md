# Features and platform consumers

An agent factory does more than construct a runtime. Its
[`module.Descriptor`](../../../backend/internal/service/agent/module/factory.go)
is the shared metadata and feature-policy record used by chat creation, prompt
orchestration, authentication, capabilities, skills, user settings, and the
frontend. Its separate provisioning profile drives host installation,
workspace provisioning, and diagnostics.

The catalog is therefore a single source of declarations, not a magic plugin
runtime: each provider still has to implement the behavior it advertises.

## Descriptor-to-consumer map

| Descriptor field | Main consumers | Effect |
| --- | --- | --- |
| `ID` | module, runtime, and auth registries; chats; settings; API routes | Stable lowercase identity for lookup, persistence, and URLs. |
| `Label` | auth/capability APIs, frontend, instruction-style skill prompts | Human-readable name; descriptor value overrides a label returned by capability probing. |
| `Default` | chat and user-settings services | Preferred default in every compatible scope. The catalog rejects multiple defaults and a default without host scope. If none is declared, the first compatible module wins. |
| `ExecutionScopes` | chat create/update/run, capability discovery, skills, profile selection | Controls whether the provider may be used for loose host chats, project chats, or both. |
| `Auth` and `AuthInstructions` | auth registry, HTTP/WebSocket routes, onboarding and Settings | Selects `managed-code`, `managed-device`, `external`, or `none` behavior. |
| `SatisfiesAccessGate` | startup validation and auth middleware | Allows an authenticated deployment to open after a managed binding authenticates, or immediately for `none`. External auth cannot satisfy the gate. |
| `LegacySkillRoots` | skill catalog | Adds provider-specific host skill locations behind the canonical `.agents/skills` root. |
| `Features.Sessions` | prompt service and chat forking | Enables saved-session resume and, separately, native fork. Fork requires resume. |
| `Features.Skills` | skill catalog and prompt preparation | Chooses no injection, slash commands, dollar mentions, or `SKILL.md` instructions. |
| `Features.BrowserTools` | capability API and prompt service | Allows the selected `browser` skill to request browser provisioning and provider launch wiring. |
| `Features.ScheduledTools` | skill catalog, prompt service, capability API/frontend | Advertises the Scheduled Tasks skill and permits issue/provisioning of a scoped schedule grant. |

The provisioning `Profile` is a separate private field of `module.Factory`,
not descriptor metadata. Only `Profiles()` and `HostProfiles()` expose cloned
policy to host/container composition. During runtime construction, the factory
uses one validated clone for its project preparer and injects an independent
clone into the provider build callback.

`Runtime.WorkspaceSkillHome(provider)` is a narrow projection derived from the
validated private profile. It lets the project skill catalog find the
provider's compatibility root without putting filesystem policy into the
public descriptor.

All descriptor and profile snapshots are defensive copies. Mutating a value
returned by `Descriptor()`, `Descriptors()`, `Profiles()`, or `HostProfiles()`
does not change the catalog.

## Current built-in declarations

| Provider | Default | Scopes | Auth | Sessions | Skills | Browser | Schedules |
| --- | ---: | --- | --- | --- | --- | ---: | ---: |
| Claude | No | host, project | managed code | resume, fork | slash command | Yes | Yes |
| Codex | Yes | host, project | managed device | resume, fork | dollar mention | Yes | Yes |
| Kimi | No | host, project | managed device | resume | instructions | No | Yes |
| Antigravity | No | host, project | external | resume | instructions | No | Yes |

All four current modules run local CLIs and attach provisioning profiles. The
contract also permits a host-only remote integration with no profile and a
no-auth module with no binding.

## Where the catalog is consumed

```mermaid
flowchart TD
    Factory["Provider NewFactory()"] --> Catalog["module.Catalog"]
    Catalog --> Build["Build one module.Runtime"]
    Catalog --> Host["HostProfiles: install-host-agents"]
    Catalog --> Project["Profiles: base image + container stack"]
    Build --> Chat["Chat provider/default/scope policy"]
    Build --> Prompt["Provider lookup + session/skill/tool policy"]
    Build --> Auth["Auth bindings + access gate"]
    Build --> Caps["Capability providers + descriptor decoration"]
    Build --> Skills["Skill roots and strategies"]
    Build --> Settings["Valid/default provider"]
    Auth --> Frontend["Generic frontend registries"]
    Caps --> Frontend
```

[`cmd/remote/main.go`](../../../backend/cmd/remote/main.go) calls the explicit
composition root once. Project profiles configure
[`config.NewContainerStack`](../../../backend/internal/config/containers.go),
and [`service.New`](../../../backend/internal/service/services.go) builds one
`module.Runtime` and injects it through each consumer's narrow interface. This
keeps registration order, identity, auth, and metadata consistent across all
views.

The same compiled catalog is also used outside the server:

- [`cmd/install-host-agents`](../../../backend/cmd/install-host-agents/main.go)
  consumes `HostProfiles()` and converges host CLIs sequentially;
- [`cmd/build-base-image`](../../../backend/cmd/build-base-image/main.go)
  consumes project `Profiles()` to build the reusable LXD image;
- [`cmd/upgrade-workspaces`](../../../backend/cmd/upgrade-workspaces/main.go)
  rebuilds the project container stack from the same profiles before replacing
  eligible containers.

A provider added only to its local package but not to
[`config.NewAgentModules`](../../../backend/internal/config/agents.go) is
inert: it will not be installed, discovered, authenticated, selected, or run.

## Identity, order, defaults, and scope

Incoming chat/settings IDs are normalized with `strings.TrimSpace` plus
lowercase. A factory's descriptor ID is not rewritten: it must already match
the grammar implemented by
[`agent.ValidProviderID`](../../../backend/internal/agent/identity.go), beginning
with `a-z` and followed only by `a-z`, `0-9`, or `-`. IDs are used in route
segments and persisted maps, so changing an ID is a data migration, not a
cosmetic rename.

[`module.NewCatalog`](../../../backend/internal/service/agent/module/catalog.go)
preserves the order from the explicit configuration builder list. That order is
used by descriptors, provisioning profiles, runtime registration, auth
bindings, capability responses, and default fallback. The catalog rejects an
empty set, duplicate IDs, multiple defaults, and cross-provider persistent
mount collisions.

`DefaultProvider(scope)` returns the explicit default when it supports the
requested scope; otherwise it returns the first compatible module. Chat
creation and user-settings defaults use it. A saved provider is still checked
for catalog membership and the chat's current scope.

Scope has concrete consequences:

- `host` allows loose-chat execution, host capability discovery, host skills,
  and inclusion in `HostProfiles()` when a profile exists;
- `project` allows project chat creation/run, project capability discovery,
  project skills, and inclusion in `Profiles()`;
- any project-scoped module must supply a complete provisioning profile;
- a host-only API adapter may omit a profile because there is no local CLI to
  install.

The prompt boundary checks scope again even though chat create/update already
validate it. This protects old or manually edited stored chat records.

## Authentication and access

Factories build the provider runtime and auth binding in one callback.
[`Factory.buildComponents`](../../../backend/internal/service/agent/module/factory.go)
checks that provider ID, auth binding ID, declared auth mode, and actual auth
flow agree.

The four supported modes are:

| Mode | Binding | Platform behavior |
| --- | --- | --- |
| `managed-code` | `auth.NewCodeBinding` | Remote starts an interactive code-paste CLI flow and exposes start/submit/cancel actions. |
| `managed-device` | `auth.NewDeviceBinding` | Remote starts a device login and exposes URL/code/progress. |
| `external` | `auth.NewExternalBinding` | Remote shows instructions only; there is no managed status stream or mutation action. |
| `none` | no binding | Provider is treated as authenticated; instructions must be empty. |

The normalized catalog is `GET /api/agent-auth`. Every non-`none` binding gets
provider-ID-derived legacy status routes; an external binding has no usable
status stream. Managed bindings additionally receive their code/device action
routes and `/ws/agent-auth/<provider>`. Route construction is generic in
[`AgentAuthHandler`](../../../backend/internal/transport/http/handlers/agent_auth_handler.go)
and [`AgentAuthSocket`](../../../backend/internal/transport/ws/agent_auth_socket.go).

An authenticated deployment calls `ValidateAccessGate` at service startup. At
least one module must declare `SatisfiesAccessGate`; managed providers are
ready only when their live binding is authenticated, and a no-auth gate is
ready immediately. External auth cannot be a gate because Remote has no
authoritative status signal for it.

The frontend's
[`useAgentAuthRegistry`](../../../frontend/src/state/hooks/auth/useAgentAuthRegistry.ts)
loads the ordered catalog and opens normalized sockets only for managed modes.
Settings/onboarding cards are generated from the returned descriptors. A
fundamentally new auth mode is therefore not “just another provider”: it needs
coordinated changes to the Go enums/validation, binding contract, handlers,
frontend types/state, and UI.

## Capability metadata and the frontend

Each runtime adapter discovers provider-native models and controls, then
[`capability.Service.decorate`](../../../backend/internal/service/agent/capability/service.go)
overwrites shared metadata from the descriptor: label, default, scopes,
authentication, sessions, skills, browser support, and schedule support. The
registered provider ID is authoritative even if the adapter returned another
or empty ID.

The capability service filters providers by execution scope and preserves
catalog order. `GET /api/agent-capabilities` supplies the frontend with all
provider/model options for the selected host/project environment. The
frontend treats provider IDs as strings and renders options from that response
through
[`agentCapabilityState`](../../../frontend/src/state/chat/agentCapabilityState.ts),
so a normal provider addition requires no provider-specific picker component.

Managed providers that are not authenticated are disabled using the auth
catalog. A capability adapter may separately return `UnavailableReason`, for
example when a CLI is installed but its project-local sign-in is absent.
Warnings do not disable a provider; they indicate partial/fallback discovery.

The frontend has a temporary built-in `codex` fallback while settings are
loading, but server-side chat and user-settings defaults come from the module
catalog. If the explicit default changes, add regression coverage for initial
frontend loading as well as the backend default path.

## Session consumers

The descriptor does not parse sessions; it tells orchestration whether an
adapter can use them.

- `Resume=false` prevents a saved provider session ID from entering
  `RunRequest`.
- `Fork=true` allows chat fork to preserve the selected provider's session and
  set `ForkPending`; validation rejects fork without resume.
- Session IDs are persisted in a provider-keyed map, so a fifth provider does
  not require a new chat field or store schema.
- The four provider-named fields in backend and frontend models are temporary
  compatibility mirrors. Do not add another named field for a new provider.
- Changing providers keeps each provider's independent session in the map, so
  switching back may resume it. Rewind clears all provider sessions.

See [`chat.Meta`](../../../backend/internal/service/chat/model.go),
[`chat.Service.Fork`](../../../backend/internal/service/chat/service.go), and
[`prompt.emitAgentEvent`](../../../backend/internal/service/prompt/agent_events.go).

## Skill consumers

[`skills.Service`](../../../backend/internal/service/skills/service.go) reads
the canonical host/project `.agents/skills` roots plus any declared legacy
roots. `Runtime.WorkspaceSkillHome` derives the provider compatibility root
from the validated profile; the skill service uses it only when it resolves
safely below `/workspace`. Hidden subdirectories are skipped and duplicate
provider/source/command entries are removed.

`Features.Skills=none` returns an empty provider skill list. Other strategies
control prompt injection, but do not install or translate provider-native
skills by themselves. The profile declares required workspace/home links, and
the factory's preparation policy plus the shared preparer must make them
usable.

When project scoped and `ScheduledTools=true`, the skill catalog adds Remote's
reserved Scheduled Tasks skill if no copy already exists. The prompt service
then issues a short-lived capability; shared project preparation publishes the
schedule CLI/skill for an enabled run.

## Browser consumers

`BrowserTools=true` makes browser support visible in capabilities and permits
the prompt service to set `RunRequest.EnableBrowser` when the user selected the
`browser` skill. That declaration alone does not connect the provider:

1. the profile may need provider-specific Browser MCP template files;
2. the factory's shared project-preparation options must request the browser
   asset and/or MCP/core paths it needs;
3. the CLI launch must receive its native MCP/config arguments;
4. provider tests must demonstrate that browser wiring appears only when
   enabled.

Claude and Codex currently select the shared preparer's full MCP/core launch
path. A module must not claim `BrowserTools` merely because the generic browser
skill exists.
The prompt service also keeps project browser activity alive once per minute
during an enabled run so the browser reaper does not stop an active session.

## Scheduled-tool consumers

`ScheduledTools=true` allows both interactive and scheduled turns to use
Remote's provider-neutral schedule tooling. The prompt service rejects this
feature in loose chats, issues the appropriate short-lived grant, places only
valid backend-issued variables in `RuntimeEnv`, and revokes the grant at the
end of the run.

Providers do not parse the grant. Shared project preparation calls
`ScheduleTools.Ensure`; the provider passes `RuntimeEnv` through the common
container-command builder. The same provider run/event pipeline is used for
interactive and scheduled turns;
the resulting chat events receive `ScheduledTaskID` at the prompt boundary.

## Provisioning and diagnostic consumers

`Profile` separates provider-owned policy from environment-owned mechanisms:

- `HostProfiles()` drives exact pinned host CLI convergence;
- project `Profiles()` drive the base image, per-run CLI repair, credential
  transfer, persistent disk devices, instruction publication, skill links,
  Browser MCP assets, lifecycle migration, and container inspection;
- `Catalog.Build` supplies application-facing project/container dependencies to
  each factory; the factory constructs shared preparation from one defensive
  profile clone and gives its provider callback only a preparer, optional
  credential collector, global sync timeout, and an independent profile clone.

The callback uses its injected snapshot only for provider-native behavior that
still needs it, such as post-run credential sync. It neither constructs shared
preparation nor calls `Profile()` again, so validated provisioning and runtime
execution cannot silently drift to different policy definitions.

Project inspection exposes generic `agents` and `authBundles` arrays. The
named Claude/Codex diagnostic fields are compatibility mirrors only. New
providers should appear through the generic arrays rather than extending the
project response with another named field.

Any new or changed CLI/profile affects host or container state. It must be
released as a minor/major full-infrastructure update; an app-only deploy does
not install host CLIs, rebuild the base image, or recycle existing workspaces.

## Declaration does not equal implementation

Before enabling a feature flag, verify both sides of the contract:

| Declaration | Required adapter/platform evidence |
| --- | --- |
| `Resume` | Native session ID is emitted, stored, and translated back into the next launch. |
| `Fork` | Native fork operation creates a different provider session without mutating the parent. |
| `Skills` | Generated trigger/path is valid for the provider's CLI and provisioned filesystem. |
| `BrowserTools` | Provider command receives working browser MCP/tool configuration. |
| `ScheduledTools` | Shared project preparation provisions the tool; the provider forwards the issued environment through the common command builder. |
| capability model/effort/tier/mode | The run adapter actually forwards every selectable value, or deliberately omits it from discovery. |
| project scope | Profile plus `ProjectPreparer` policy and provider command can prepare, run, and preserve state in a project container. |
| host scope | Host auth, command environment, cwd, and CLI availability are supported. |

This distinction matters for future agents: passing factory validation proves
that the declaration is structurally coherent. Provider tests prove that the
native integration actually honors it.
