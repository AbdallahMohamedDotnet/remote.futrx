# Change password and sign out other sessions

**Issue:** [#374](https://github.com/futrx-com/remote.futrx/issues/374)
**Status:** in-progress (preflight amendments approved by requester on 2026-10-16)
**Tier:** structural, small. It adds one API route, one field on an existing persisted record, and one store method. New files within existing packages only; no new package, dependency or `infra/` change, so it is a PATCH-class release.
**Blocks:** [#372](https://github.com/futrx-com/remote.futrx/issues/372). Plan: `docs/plans/2026-10-16-email-invitations.md`. It reuses the change rules and session revocation built here.

**Scope (one sentence):** An account that signs in with a password can change it from Settings → Security after proving the current password, plus 2FA if enabled. Every other session of that account then stops working, while the browser that made the change stays signed in.

**Out of scope:** forgot-password by email; administrator-initiated reset (#372); invited password accounts (#372, which extends this flow to them); changes to Google OAuth; a "sign out everywhere" button that doesn't change the password; encrypting `DATA_DIR` at rest.

---

## Requirements record

### Confirmed decisions (requester, 2026-10-16)

| # | Decision |
| --- | --- |
| D1 | Self-service password change applies to **every account that has a password**: the local administrator now, and invited accounts once #372 lands. |
| D2 | The user must enter the **current password**. If 2FA is enabled, they must **also** enter an authenticator code or recovery code, the same as disabling 2FA today. |
| D3 | A password change **signs out every other session** of the account. The browser that made the change gets a fresh session. |
| D4 | The password rule is unchanged: at least 12 Unicode characters and at most 1024 bytes, no composition rules. The new password must differ from the current one. |
| D5 | It ships as its own PR, ahead of #372. |

### Evidence about existing behaviour (unchanged unless listed above)

- Sessions are stateless HMAC tokens `{email, sub, iat, exp, sid}` (`service/auth/model.go` `Session`). `iat` is in **seconds**.
- `CurrentSession` (`service/auth/service.go:354`) is the single place every gate checks a session: the API middleware, `/auth/verify` and the WebSocket upgrades all go through `PrincipalResolver`. It already calls `sessionRegistry.IsActive` (`session_registry.go:164`), which reads a lazily cached per-account record. So adding a check there costs no extra disk read per request.
- `IsActive` **fails open** on a store read error (`session_registry.go:166`). This plan keeps that behaviour (see Risks R3).
- The local admin credential is created with `CreateLocalAdmin`, which refuses to overwrite, and removed with a compare-and-delete `DeleteLocalAdmin` (`stores/fileauth/store.go:255-300`). Nothing can replace it today.
- 2FA proof of possession with replay protection and recovery-code consumption is `twoFactorAuthenticator.VerifyChallenge` (`service/auth/twofactor.go:164`).
- The local-login limiter (`handlers/auth_local_handler.go:113-160`) allows 5 failures per key per 5 minutes.
- There are **no** handler tests for `security_handler.go`.

---

## Design

### Where each responsibility goes

| Responsibility | Owner | Why there |
| --- | --- | --- |
| "Is this session still valid?" incl. revoke-before-time | `sessionRegistry` (extend) | It already owns per-account session validity (`IsActive`, `Revoke`), its cache and its per-account lock. A new store would duplicate the cache and lock. |
| Verifying and replacing the local admin password | `LocalAdminAuthenticator` (extend) | It owns the credential, its in-memory copy and its claim lock. The change must update the store and the in-memory copy together. |
| Sequencing (verify → 2FA → revoke → save → reissue) | `auth.Service.ChangePassword` (new facade method) | It follows the facade pattern of `CompleteTwoFactorDisable` (`security.go:87`), which composes the authenticators. |
| HTTP: body, rate limit, cookie, error mapping | `SecurityHandler` (`/api/me/security/*`) | It already owns the signed-in user's own security routes. |
| Atomic replace on disk | `fileauth.Store.ReplaceLocalAdmin` (new) | Compare-and-swap, mirroring `DeleteLocalAdmin`. |
| Form rules (length, confirmation, differs) | `frontend/src/state/hooks/auth/passwordChangeFormState.ts` (new, pure) | Same shape as `localAuthFormState.ts`. Keeps components free of policy and can be tested with `node --test`. |

No new interfaces, packages or layers. `LocalAdminStore` gains one method. That is the smallest contract change that lets the change be atomic.

### Behaviour

`POST /api/me/security/password` with body `{currentPassword, newPassword, code?}`, behind a session:

1. The handler resolves the session (401 if none) and rate-limits on the key `<clientIP>|<email>|password-change`. It allows 5 failures per 5 minutes and returns 429 with `Retry-After` when exceeded, using the same limiter type as login.
2. `auth.Service.ChangePassword(ctx, user, PasswordChange{Current, New, Code}, ip, ua)` holds the local authenticator claim lock across verification, revocation, replacement and reissue, so a concurrent loser cannot revoke the winner’s fresh session:
   1. The account has no password credential (a Google-only account) → `ErrPasswordSignInUnavailable` → **409**.
   2. Verify `Current` against the stored hash. A mismatch gives `ErrCurrentPasswordIncorrect` → **403**, which counts as a failure. It must not be 401, because `frontend/src/api/apiRequest.ts:12` treats any 401 as "signed out" and would drop the user to the login screen. The password is checked before the 2FA code, so the 2FA state of the account is revealed only to someone who already knows the password.
   3. Load 2FA state with error propagation (a read failure returns 500 `password not changed`, never treating 2FA as disabled). If 2FA is enabled: an empty `Code` gives `ErrTwoFactorCodeRequired` → **400**. Otherwise call `VerifyChallenge`. An invalid or replayed proof gives **403** and counts as a failure; persistence errors give **500** `password not changed`. A recovery code used here is consumed, the same as when disabling 2FA.
   4. `New == Current` → `ErrPasswordReuse` → **400**. `HashPassword(New)` returns the existing too-short or too-long errors → **400**.
   5. **Revoke first:** `registry.RevokeSessionsBefore(email, now)` sets `SessionsValidAfter = now.Unix()`. On failure, return **500** "password not changed". Nothing else has happened yet.
   6. **Then save:** `local.replacePassword(expected, newHash)` does a compare-and-swap on disk, then updates the in-memory copy. If a concurrent change won, return `ErrLocalAdminCredentialChanged` → **409**. On another failure → **500** "password not changed; your other sessions were signed out". That is true and safe: the old password still works and the user can sign in again.
   7. **Then reissue:** `IssueSession(user, SignInMethodPassword, ip, ua)`. Its `iat` is at least the cut-off, so it passes. If reissue fails after replacement, return **500** `password changed; sign in again with your new password`, never claiming the password is unchanged.
3. The handler sets `remote_session` and returns **200** with `Status`.

`CurrentSession` → `registry.IsActive(ctx, *session) error` (nil means active; separate sentinels distinguish revoked and superseded sessions). Cold registry loads must recheck the cache under its write lock before publishing their result, so a stale load cannot overwrite a newer revocation. The order is: record missing → active; `session.Iat < record.SessionsValidAfter` → `ErrSessionRevoked`; then the existing single-session rule. Accounts whose record has `SessionsValidAfter == 0` behave exactly as today.

`SecuritySummary` gains `passwordSignIn bool`, which is true when the account has a password credential. The frontend shows the card only when it is true.

### Error contract

| Condition | Status | Message (safe, no secrets) |
| --- | --- | --- |
| No session | 401 | `authentication required` |
| Google-only account | 409 | `this account does not sign in with a password` |
| Wrong current password | 403 | `current password is incorrect` |
| 2FA code missing | 400 | `two-factor code required` |
| 2FA code wrong | 403 | `invalid two-factor code` |
| Same as current / too short / too long | 400 | existing or new sentinel text |
| Rate limited | 429 | `too many attempts; try again in a few minutes` |
| Concurrent change | 409 | `password was changed elsewhere; reload and try again` |
| Revoke failed | 500 | `password not changed` |
| Save failed after revoke | 500 | `password not changed; other sessions were signed out` |
| 2FA load/save failed before revoke | 500 | `password not changed` |
| Reissue failed after save | 500 | `password changed; sign in again with your new password` |

---

## Steps

Every backend step ends with `gofmt -l backend` (empty), then `cd backend && go vet ./... && go test ./internal/...`. Frontend steps end with `cd frontend && npm test && npm run build`. Commit after each step with `git commit -s`, and with **no** co-author trailer.

0. **Characterization tests (no behaviour change).** Add `handlers/security_handler_test.go` covering what the change flow relies on: 401 without a session on `/api/me/security`, and the 2FA disable happy and wrong-code paths. Add `session_registry_test.go` cases pinning today's `IsActive` behaviour: no record → true; single-session off → true; superseded sid → false; store error → true.
   Verify: the new tests pass on the unchanged code.

1. **Session registry: revoke-before.** Files: `service/auth/model.go` (`SessionRegistryRecord.SessionsValidAfter int64 \`json:"sessionsValidAfter,omitempty"\``, `ErrSessionRevoked`), `session_registry.go` (`RevokeSessionsBefore`, `IsActive(ctx, Session)`), `service.go` (`CurrentSession` call site).
   `RevokeSessionsBefore` takes the account lock, loads or creates the record (keeping preferences, history and alert), sets the field only if the new value is larger, saves, then updates the cache.
   Verify: step-0 tests still pass. New cases: a session with `iat` before the cut-off gives `ErrSessionRevoked`; `iat` equal to it is accepted; a record created only by revocation leaves `Preferences` all false, so `IssueSession` still skips the registry; logout `Revoke` and `SetPreferences` keep `SessionsValidAfter`; a controlled cold-load/revoke race cannot restore stale cached data.

2. **Store: replace the local admin credential.** Files: `service/auth/ports.go` (`ReplaceLocalAdmin(ctx, expected, next LocalAdminCredential) error` on `LocalAdminStore`), `stores/fileauth/store.go` + test, and every fake implementing `LocalAdminStore` (`grep -rn "DeleteLocalAdmin(" backend --include=*_test.go`).
   Use the same normalization and incomplete-credential checks as `DeleteLocalAdmin`. If the file is missing or doesn't match `expected`, return `ErrLocalAdminCredentialChanged`. Write with `writeJSONLocked` (0600, temp + rename).
   Verify: success; mismatch → sentinel and the file is unchanged byte for byte; missing → sentinel; mode stays 0600.

3. **Authenticator + service.** Files: `local_admin_authenticator.go` (`verifyPassword(email, password) (credential, error)` and `replacePassword(ctx, expected, hash)` with the workflow holding `claimMu`), `password_change.go` (new: `PasswordChange`, `ChangePassword`, sentinels `ErrPasswordSignInUnavailable`, `ErrCurrentPasswordIncorrect`, `ErrTwoFactorCodeRequired`, `ErrPasswordReuse`) + `password_change_test.go`, and `security.go` (`SecuritySummary.PasswordSignIn`).
   Verify, table-driven, with fakes that record call order:
   - each error row above;
   - the exact order revoke → save → reissue;
   - a failed revoke means **no** save;
   - a failed save after revoke returns the specific sentinel;
   - the old password fails and the new one works through `CompletePasswordLogin`;
   - a session issued before the change gets `ErrSessionRevoked` from `CurrentSession`, and the reissued one passes;
   - a recovery code is consumed;
   - 2FA load/save failures never skip proof or revoke sessions;
   - failed reissue returns the changed-password sentinel; concurrent changes with the same current password have exactly one winner and keep its fresh session valid;
   - a capturing `slog` handler and every error string contain neither password.

4. **HTTP.** Files: `handlers/security_handler.go` (route, limiter field, error mapping), `security_handler_test.go`, `transport.go:122` (pass a `newLocalLoginLimiter()`).
   Verify: each status in the error contract through `httptest`; a successful change sets `remote_session` with the same attributes as `setSessionCookie`; the old cookie then gets 401 on `/api/projects` through the real middleware; `/api/me/security` returns `passwordSignIn: true` for the local admin and `false` for a Google user.

5. **Frontend.** Files: `config/routes.ts`, `api/securityApi.ts` (`changePassword`), `models/security` type (`passwordSignIn`), `state/hooks/auth/passwordChangeFormState.ts` + `.test.ts`, `state/hooks/auth/useSecuritySettings.ts` (expose `changePassword`), `ui/settings/security/PasswordChangeForm.tsx` (new), `ui/settings/SecuritySettings.tsx` (render when `passwordSignIn`).
   The form has current password, new password, confirmation, and a code field shown only when `twoFactorEnabled`. Use `autocomplete` values `current-password`, `new-password`, `one-time-code`. Show success text after the change. Errors show the server message. Clear the fields on success and on unmount.
   Verify: form-state tests cover length < 12, mismatch, same-as-current, and the code required when 2FA is on; `npm run build`.

6. **Docs.** `ARCHITECTURE.md` "Sessions" paragraph (`per-request IsRegistered checks are the only way a session is invalidated early` → add password change), `docs/02-workspaces/02-auth-users-and-access.md`, `docs/03-platform/08-api-and-realtime.md` (new route), `docs/threat-model.md` (stolen-session window now closes on password change), and the README user guide (Settings → Security).
   Verify: `grep -rn "sessionsValidAfter\|me/security/password" ARCHITECTURE.md docs/` hits each file.

7. **Full verification.** `gofmt -l backend`, `cd backend && go vet ./... && go test ./...`, `go test ./...` at the repo root, `cd frontend && npm test && npm run build`. Then `bash infra/qa/deploy-app.sh <sha>` (no infra change, per `AGENTS.md`) and the acceptance criteria below on QA.

## Data / migrations

- The session registry record (`DATA_DIR/sessions/`, `stores/filesessions`) gains an optional `sessionsValidAfter`. A missing value means 0, which means no revocation. Old records stay valid. An older binary ignores the field. On rollback, revoked sessions become valid again until they expire. Record this in the release note.
- `local-admin.json` keeps its shape. It is replaced atomically.

## Risks

| # | Risk | Mitigation |
| --- | --- | --- |
| R1 | `iat` has 1-second resolution. A session issued in the same second as the change, before it, survives. | Accepted and documented. This includes stolen existing cookies issued in that same second, regardless of how the attacker obtained them. |
| R2 | A revoke is written but the save fails, so the user is signed out everywhere and the password is unchanged. | Safe failure, and the message says so. |
| R3 | `IsActive` fails open on a registry read error, so a revoked session could pass while the disk errors. | This existing behaviour is frozen here and noted in the threat model. Changing it would affect every request and needs its own decision. |
| R4 | Using a recovery code to change the password doesn't raise the recovery-code alert. | Out of scope. Noted as a follow-up. |

## Acceptance criteria (Given / When / Then)

1. **Given** the local admin is signed in on browsers A and B, **when** they change the password on A, **then** A stays signed in and B's next request returns to the login screen.
2. **Given** 2FA is on, **when** the code is missing or wrong, **then** nothing changes and the error is shown.
3. **Given** a wrong current password five times, **then** the sixth attempt returns 429.
4. **Given** a successful change, **then** the old password fails at login and the new one works.
5. **Given** a Google-only member, **then** Settings → Security shows no password card, and the API returns 409.
6. **Given** any attempt, **then** `journalctl -u remote.futrx` and `DATA_DIR` contain neither password in plain text.
7. **Given** an account that never changed its password, **then** the sign-in flow, single-session and history behave as before.

## Verification gate

Read-only delegated preflight checked the reference implementations and all step verification commands at base `37672a53`. Requester approved the following design amendments: error-propagating 2FA loading, truthful post-save errors, serialized password-change workflows, and protection against stale registry cache publication. The IsActive error result is necessary to express the specified sentinels. No dependency or infrastructure changes are needed. Preserve existing fail-open registry reads, the accepted same-second window, and current login behavior.

**Baseline:** backend `go vet ./... && go test ./...`, root `go test ./...`, frontend `npm test` (101 tests) and `npm run build` passed. Backend tests required local sockets unavailable under the initial sandbox; rerunning outside it passed. Existing Vite configuration/chunk-size warnings remain.

**Additional follow-up:** pending 2FA login tokens issued before a change can still complete afterward; invalidating pending login proofs is outside this session-cookie scope.

## Progress

- [x] 0. Characterization tests — `verified`: new HTTP and registry tests pass unchanged; gofmt empty, backend vet and internal suite pass.
- [x] 1. Session registry — `verified`: cutoff boundaries, monotonic preservation, failure paths, untracked issuance and controlled stale-load races pass; focused race tests ×10, backend vet/internal suite, gofmt pass.
- [ ] 2. Credential store
- [ ] 3. Authenticator and service
- [ ] 4. HTTP
- [ ] 5. Frontend
- [ ] 6. Documentation
- [ ] 7. Full verification and QA deployment

## Outcome

<!-- filled after implementation -->
