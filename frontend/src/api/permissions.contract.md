PROPOSED contract. Not implemented by the backend. Pending sign-off from whoever
writes the backend routes. If the backend picks different paths or shapes, only
permissions.contract.md and permissions.ts change; the UI does not.

Status legend: **verified** = visible in `backend/internal/rbac`; **proposed** =
the transport layer does not exist, so the HTTP shape is a proposal;
**unverified** = not visible in the code.

## Types (field names from `backend/internal/rbac/models`)

Only `Scope` carries json tags in Go (`kind`, `id,omitempty`). `Definition`,
`Role`, `RoleRule`, `RoleBinding` and `Assignment` have **no json tags**, so
their wire casing is **unverified**. This contract proposes lowerCamelCase of the
Go field names (`ID` -> `id`, `UserEmail` -> `userEmail`, `RoleID` -> `roleId`).

| Type        | Fields                                                                                  |
|-------------|-----------------------------------------------------------------------------------------|
| Scope       | `kind` ("platform" \| "project"), `id?` (platform takes no id; project id `^[a-z0-9][a-z0-9_-]{0,63}$`) |
| Definition  | `key`, `description`, `scopes: ScopeKind[]`, `baseline` ("none" \| "admin" \| "project-member" \| "authenticated"), `delegable` |
| RoleRule    | `permission`, `effect` ("allow" \| "deny")                                              |
| Role        | `id`, `name`, `description`, `rules: RoleRule[]`, `createdBy`, `createdAt`, `updatedAt` |
| Assignment  | `id`, `userEmail`, `permission`, `effect`, `scope`, `createdBy`, `createdAt`            |
| RoleBinding | `id`, `roleId`, `userEmail`, `scope`, `createdBy`, `createdAt`                          |

Timestamps are Unix milliseconds (`UnixMilli()` in the service) — verified.
Ids are server-assigned (`newID()`) — verified.

## Reads

```
GET /api/admin/permissions/definitions  -> 200 { definitions: Definition[] }, 403 denied
GET /api/admin/permissions/roles        -> 200 { roles: Role[] }
GET /api/admin/permissions/assignments  -> 200 { assignments: Assignment[] }
GET /api/admin/permissions/bindings     -> 200 { bindings: RoleBinding[] }
```

Backend currently returns 500 for denials until it maps ErrDenied to 403. The UI
treats any non-2xx as an error and does NOT special-case 403 yet.

Definitions are code-owned (registry). GET only; there are no write routes and
there never will be.

## Writes

```
POST /api/admin/permissions/assignments
  body { userEmail, permission, scope: { kind, id }, effect }
  Upsert: same effect is a no-op, different effect updates. Returns the assignment.
  201 created / 200 updated / 400 invalid key, scope or unregistered user / 403 denied

DELETE /api/admin/permissions/assignments/{id}  -> 204, idempotent

POST /api/admin/permissions/bindings
  body { userEmail, roleId, scope: { kind, id } }
  Idempotent. Returns the binding.
  201 created / 200 existing / 400 if role rules do not support scope kind, or user unregistered / 403 denied

DELETE /api/admin/permissions/bindings/{id}  -> 204, idempotent

POST /api/admin/permissions/roles
  body { name, description, rules: [{ permission, effect }] }
  Server-assigned id.

PUT /api/admin/permissions/roles/{id}
  body { name, description, rules }

DELETE /api/admin/permissions/roles/{id}?unbind=true
  204 / 409 if bound and unbind not set.
```

## Verified vs unverified

Verified in `backend/internal/rbac` (service layer):
- `SetAssignment`: same effect is a no-op, a different effect updates the existing record (`assignments.go`).
- `BindRole`: binding the same (user, role, scope) twice is a no-op (`assignments.go`).
- `RemoveAssignment` / `UnbindRole` / `DeleteRole` of a missing record is a no-op.
- `DeleteRole` on a bound role fails with `ErrRoleInUse` unless `Unbind` is set; with it, bindings are removed with the role (`roles.go`).
- Role validation: name 1-80 chars and unique case-insensitively, description <= 500, at least one rule, no duplicate permission, known permission, valid effect (`roles.go`).
- `BindRole` rejects roles whose rules do not support the scope kind (`requireRulesSupportScope`).

Proposed, not confirmed (no transport exists):
- Every path, status code and body wrapper above, including 201 vs 200 (the service does not report created vs existing) and 409 for `ErrRoleInUse`.
- Removal by `{id}`: the service removes by natural key (user, permission, scope) or (role, user, scope), so the handler must resolve the id first.
- `PUT` for role update, and a 400 for `ErrUserNotRegistered`.

Unverified:
- JSON casing of every record except `Scope`.
- Which HTTP status `ErrDenied` produces today (no mapping found in `transport/`).
