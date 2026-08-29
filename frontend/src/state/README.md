# state/

Four folders, each answering one question. Every top-level entry here is a
**role**; the domain split lives one level down.

| Folder | Its one job | Holds state? |
| --- | --- | --- |
| `stores/` | State that must outlive the component tree | **Yes** — this is the whole list |
| `logic/<domain>/` | Projectors, policies, reducers, builders | No — computes from its arguments |
| `hooks/<domain>/` | The access layer: effects, API calls, lifecycle | In preact hooks |
| `context/` | Cross-cutting state, gated in nesting order | In preact hooks |

## The access rule

**State is read only through `hooks/`.** Nothing in `ui/` or `app/` may
subscribe to or read from a store — a store outlives the component tree, so a
direct read misses every later change and never re-renders. `hooks/` and
`context/` are the only importers of `stores/`.

**Commands may be dispatched from anywhere.** Writing to a store is not a
subscription and carries no re-render obligation. This matters because some
dispatch sites are not components and cannot call a hook — see
`ui/chat/markdown/inlineParser.tsx`, which opens the media viewer from a link
handler inside a vnode builder. That is the one file outside `hooks/` and
`context/` that imports a store, and it is deliberate.

## Naming

- `*Store` — keeps mutable fields. If you add one, it belongs in `stores/`.
- `*State` — keeps nothing; a policy, reducer, or projection over its arguments.

The suffix used to be decorative: three stores were named `*State` while their
stateless siblings used the same suffix. It now carries information, so keep it
honest.

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

Every test file in `state/` is in `logic/`. `hooks/` and `context/` have none —
there is no hook or component test harness in this repo, so the compiler and
the build are the only net there.

That is the reason for the split, not a side effect of it. Logic that can be
pulled out of a hook becomes testable by being pulled out. When a hook grows a
rule worth pinning — a fallback, a convergence condition, a mapping — move the
rule to `logic/` and let the hook keep the lifecycle.

## Layering

`ui → app → state → api → transport`, with `config`, `models` and `shared` as
leaves anyone may import. `state/` imports nothing from `ui/`.
