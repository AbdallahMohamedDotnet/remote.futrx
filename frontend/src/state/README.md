# state/

Three folders, each answering one question.

| Folder | Its one job | Holds state? |
| --- | --- | --- |
| `stores/<domain>/` | Global state, and everything that keeps it | **Yes** — this is the whole list |
| `hooks/<domain>/` | The access layer: how UI reads a store, plus state owned by one screen | In preact hooks |
| `context/` | Cross-cutting state, gated in nesting order | In preact hooks |

`stores/` and `hooks/` both group by domain, in the same words `ui/` and
`services/` use: **agents, chat, media, push, workspace**. The store-to-store
edges all sit inside one folder — the capability catalog and its wiring, both
push stores and `pushPageFocus`, the workspace store and its projector — so
the folders are where the coupling already was, not a grid imposed over it.

`media/` is the one domain with no matching `hooks/` folder.
`mediaViewerStore` is opened from file-manager rows and from chat links alike,
and filing it under whichever domain holds today's callers would claim an
ownership it does not have.

## Pure functions are not a layer

There used to be a fourth folder, `logic/`, holding the projectors, policies
and reducers. It answered no question of its own: half of it was the private
insides of one hook or store, and half was pure helpers that `ui/` imported
directly. Neither half is a layer, so there is no folder for them now.

**A module lives with its owner.** `workspaceDataProjector` computes the next
workspace snapshot and nothing else imports it, so it sits in `stores/` beside
`workspaceStore`. `chatEventStateProjector` belongs to `useChat` and sits in
`hooks/chat/`. `createProjectForm` is validation for one modal and lives in
`ui/projects/` next to it. Splitting a rule out of the thing that owns it is
still worth doing — it is what makes the rule testable — but the split is a
file, not a directory.

**A module with owners in more than one layer goes to `services/`.**
`usageFormatService` is read by four components, `workspaceSidebarService` by
`app/`, `ui/`, two hooks and a context. They own no state and answer to no
single caller, which makes them leaves — the same category as `config/` and
`models/`. Each one is a class with a single exported instance; see
[`../services/README.md`](../services/README.md).

## The access rule

**Global state is read only through `hooks/`.** Nothing in `ui/` or `app/` may
subscribe to or read from a store — a store outlives the component tree, so a
direct read misses every later change and never re-renders. `hooks/` and
`context/` are the only importers of `stores/`.

This is about *global* state, not all state. Roughly half the hooks here own
something local — a date range, a textarea's height, a drag in progress — and
those should stay local. Promoting a form's fields to a store to keep the
folder count tidy is the failure this rule exists to prevent.

**Commands may be dispatched from anywhere.** Writing to a store is not a
subscription and carries no re-render obligation. This matters because some
dispatch sites are not components and cannot call a hook — see
`ui/chat/markdown/inlineParser.tsx`, which opens the media viewer from a link
handler inside a vnode builder. That is the one file outside `hooks/` and
`context/` that imports a store, and it is deliberate.

## Naming

- `*Store` — keeps mutable fields. If you add one, it belongs in `stores/`.
- `*State` — keeps nothing; a policy, reducer, or projection over its arguments.

The suffix carries the placement rule, so keep it honest. `promptQueueState`
sits in `hooks/chat/` because it holds nothing; `pushPresenceStore` sits in
`stores/push/` because it holds the chat this client has claimed — global state
that never renders is still global state.

Two files in `stores/` carry neither suffix, and both are honest about it.
`stores/push/pushPageFocus.ts` is a three-line read of `document`, private to
the two push stores that call it. `stores/agents/agentCapabilityCatalog.ts`
keeps no state either — it is the one line that constructs
`AgentCapabilityCatalogStore` with the API function it depends on, kept apart
from the class so the class can be built against a fake in its test.

## Where the types and the constants live

**Not here.** A data shape belongs in `models/` and a tunable belongs in
`config/`, whichever module happens to compute or consume it. `ChatRenderState`
is declared beside `ChatMeta`, not inside the projector that builds it; the
agent-browser poll interval sits in `config/agents.ts`, not in the hook that
passes it to `setTimeout`.

What stays is what describes one module's own insides and never appears in an
import elsewhere: a store's listener signature, a reducer's private bucket, a
hook's options bag. Exporting those would widen the surface for nothing.

The one deliberate exception is a contract that carries behaviour rather than
data — `useUsageDashboard`'s return type, `ConfirmOptions` with its preact
children and its action callback. Those are published by the hook or context
that produces them, because that is what they describe.

## Where the tests are, and why

Beside the module they test, which now means throughout `state/` rather than in
one folder. `hooks/` and `context/` still have no test for a hook or a
component — there is no harness in this repo, so the compiler and the build are
the only net for those.

That is still the reason to pull a rule out of a hook: `promptQueueState.ts`
has a test and `usePromptQueue.ts` cannot. When a hook grows a fallback, a
convergence condition, or a mapping worth pinning, move the rule into its own
file and let the hook keep the lifecycle. The file lands next to the hook, so
the move costs nothing but the import.

## Layering

`ui → app → state → api → transport`, with `config`, `models` and `services`
as leaves anyone may import. `state/` imports nothing from `ui/`.
