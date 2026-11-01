# PR #223 integration and refactor

Scope: the permission, provider-account authorization, operational audit, and
administration UI changes introduced by PR #223. Unrelated QA code was retained.
The refactor baseline is `ec404a87`, after merging QA; permission decisions,
error behavior, persisted formats, audit ordering, and UI behavior are frozen
against that baseline.

## Branch integration

- Removed the PR's tracked `.claude/` directory. Also removed its five
  `.opencode/skills` symlinks and `opencode.json`, whose only configuration
  pointed to the removed skills. Neither directory/configuration exists in QA.
- Merged `origin/qa` at `a736a2d6` with a merge commit, preserving both histories.
- Resolved the deleted `file_manager_handler.go` in favor of QA's move to the
  installable File Manager application. The removed core handler and Files
  toolbar action were not restored. Git-history and browser permission checks
  remain on the surviving toolbar actions.
- The PR branch remains open; this work does not merge it into QA or deploy it.

## Smell → principle → change

Line references in the **before** column refer to `ec404a87`.

| Before | Principle | Change |
| --- | --- | --- |
| `backend/internal/rbac/service.go:111` gave each locked mutation the entire service | Encapsulation and narrow dependencies | `rbac/mutation.go` owns its state/events and explicit registry, identity, evaluator, clock, and ID dependencies; the repository lock stays in `Service.mutate`. |
| `rbac/assignments.go:252` owned role-scope compatibility also used by role updates; registry repeated scope errors | One owner for catalog validation | Registry owns scope support checks; `rbac/permissions.go` owns the management catalog. |
| `service/project/permissions.go:69`, `service/chat/permissions.go:34`, and `service/workspaceide/permissions.go:27` repeated fallback actor policy | Single policy owner | `rbac.SystemOnlyAuthorizer` supplies the identical missing-actor/user/system behavior. Workspaceaccess's distinct nil-authorizer denial remains unchanged. |
| `service/audit/service.go:235` encoded HTTP JSONL | Service/transport separation | `Service.VisitQuery` traverses typed entries; the HTTP handler owns JSON encoding and download headers. |
| `stores/fileaudit/store.go:407` kept entries and line positions in parallel arrays | Invariant ownership | A private `positionedEntry` keeps the pair together; pagination retains only its last cursor. |
| `service/prompt/service.go:380` mixed audit schema and completion recording into execution | Separate audit policy from run orchestration | `prompt/audit.go` owns payload construction and event recording; the runner keeps the original call/defer positions and account rechecks. |
| `frontend/src/state/hooks/permissions/usePermissionAdministration.ts:33` combined API calls, repeated mutation workflows, DOM events, and state | Service/store/hook ownership | An injected administration service owns mutation → notification → refresh; a per-SettingsContainer store owns data; the hook subscribes and delegates. |
| `state/hooks/permissions/permissionForms.ts` and `permissionBranches.ts` contained policy imported by presentation | Policy belongs in services | Form and delegation services own those rules; public types and catalogs have model/config owners; server-conflict interpretation belongs at the API boundary. |
| `ui/settings/permissions/PermissionFormFields.tsx:2` fetched account targets; effective-permission hook combined transport and UI lifecycle | Explicit I/O boundary | App composition injects permission lookup; hooks retain per-view snapshots, timers, disposal and stale-response guards. |
| `state/hooks/admin/useAuditLog.ts:28` combined API calls, pagination, stale-request policy, and view state | Service/store/hook ownership | A per-mount audit service and store own requests and data; the hook preserves render-captured inputs and effect timing. |
| `api/auditApi.ts:4` depended on a state module mixing query serialization and paging rules | Dependency direction | API-owned query serialization and service-owned page combination replace that cross-layer dependency. |

## Behavior preserved

- RBAC actor and delegation error precedence, exact scope matching, administrator
  recovery, mutation checks under the repository lock, clock/ID call counts,
  audit-before-publication, idempotence, and rollback on mandatory audit failure.
- Operational audit JSON escaping, newest-first filtered exports, raw-store
  export ordering, bounded paging, original cursor behavior, and cancellation/
  error sequencing. A 501-entry characterization crosses the export page boundary.
- Prompt start → submit → account use → completion order, application/account
  correlation, fixed failure redaction, cancellation, and completion defer order.
- Permission UI per-mount lifetime, request order, notify-before-refresh,
  mutation rejection identity, failed-refresh data retention, and existing
  polling/stale-response/disposal behavior. Audit request-sequence handling,
  render-captured paging inputs, retained results/errors and existing loading-flag
  quirks are pinned by six deferred-request tests.

## Verification

- Backend: `go vet -p 4 ./...`, `go test -p 4 ./...`, and `go build -p 4 ./...` passed.
- Backend race checks passed for RBAC/evaluator, policy/audit stores, audit,
  project, chat, prompt, account-access, HTTP handlers/middleware, and WebSocket
  packages. The shared fallback move was also checked through service composition.
- Application catalog (repository-root Go module): `go vet ./...`,
  `go test ./...`, and `go build ./...` passed.
- Code Server installer and File Manager UI Node tests: 32 passed, 4 opt-in
  browser tests skipped.
- All 17 infrastructure/release shell test scripts listed in
  `.github/workflows/ci.yml` passed.
- Frontend: `npm test` passed all 609 tests and `npm run build` passed.
  Existing Vite configuration/chunk-size warnings remain non-fatal.
- `gofmt -l backend catalog.go applications` returned no files; the task diff
  passed `git diff --check`.
- Independent reviews of permission UI, audit UI concurrency, RBAC integration,
  prompt audit recording, and audit export/pagination found no refactor regressions.

## Commit inventory

All changes are committed as separate coherent moves. Each refactor passed its
focused check before commit; the full checks above cover their combination.

| Commit | Change | Focused validation |
| --- | --- | --- |
| `3dbef118` | chore: remove PR-local Claude skills and dependent links | Tracked-path inspection and diff check |
| `ec404a87` | Merge origin/qa into permission abstraction | Frontend build + 597 tests; RBAC/store/project/audit/HTTP tests |
| `1352fb0c` | refactor(frontend): separate audit queries from paging policy | TypeScript and existing audit query/paging tests |
| `c72c3e34` | test(audit): pin filtered export paging and encoding | Audit/store/HTTP tests against the pre-refactor export |
| `ea7eccda` | refactor: isolate locked permission mutation dependencies | RBAC/policy-store tests and architecture scan |
| `7179833f` | refactor(audit): move filtered export encoding to HTTP | Audit/store/HTTP tests |
| `caf89ca3` | test(prompt): pin audit event order and failure redaction | Existing prompt package tests before extraction |
| `6d189803` | refactor: centralize permission catalog validation | RBAC/policy-store tests with race detector |
| `d17087d2` | refactor(audit): keep log entries with their cursor positions | Audit/store/HTTP tests and vet |
| `f7f9ffc2` | refactor(frontend): isolate permission administration workflows | TypeScript and workflow/store tests |
| `10cd21e2` | refactor(prompt): separate run audit recording from execution | Prompt and audit package tests |
| `fc82f138` | refactor: share the system-only authorization fallback | RBAC/project/chat/composition/HTTP tests and race checks |
| `31f73463` | refactor(frontend): separate permission policies from presentation | TypeScript, 603 frontend tests, and production build |
| `8c5a6c7d` | refactor(frontend): isolate audit paging workflow and state | TypeScript, 17 focused tests, and independent concurrency review |

## Deliberately unchanged and limits

- Existing evaluator/models/ports organization and the atomic policy store are
  already cohesive; no repository-wide package renaming was imposed.
- Existing authorization semantics and documented rollout limitations were not
  changed. Provider-account grants still require explicit provisioning; active
  editor/shell sessions and in-flight uploads retain the existing revocation
  limitations described in `docs/dev/permissions.md`.
- The service's project-attribution fallback and the file store's legacy
  metadata fallback differ for an empty project target. That existing behavior
  was preserved rather than unified accidentally.
- Audit UI export continues to filter by dates only, although the HTTP export
  endpoint accepts additional filters. Expanding UI export is a feature change.
- `PermissionsHandler.HandleEffective` still discovers its optional capability
  by type assertion; replacing that public handler contract could change its
  existing unavailable response and was not required for these structural moves.
- No QA VM deployment, live browser interaction validation, provider login, or
  real LXD operation was performed. Compilation and unit/race tests do not prove
  those external runtime paths. Four opt-in File Manager browser tests skip
  without `FUTRX_FILE_MANAGER_URL`.
- No future or recurring job was requested, so no schedule was created.
