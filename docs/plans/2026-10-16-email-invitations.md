# Email invitations with temporary credentials

**Issue:** [#372](https://github.com/futrx-com/remote.futrx/issues/372)
**Status:** planned
**Tier:** structural. It adds a password credential for invited accounts, a forced first-login password change, a new `DATA_DIR` file, and a new admin flow. It adds no dependency and changes nothing under `infra/`, so it is a PATCH-class release.
**Scope:** An administrator creates an account from an email address and an existing role. Remote generates a temporary password and emails it when SMTP is configured, or shows it with a PDF download when SMTP is not. The invited user must replace that password before any session exists. Google OAuth keeps working as before.
**Out of scope:**
- Self-signup, password reset by email ("forgot password"), and changing an existing user's sign-in method in bulk.
- New roles. The invite form uses the existing `admin` and `member` roles.
- Changes to the local-admin claim flow, setup token, Google OAuth configuration, or 2FA.
- Encrypting `DATA_DIR` at rest. The new file uses the existing plaintext, mode 0600 posture (threat model finding 18). It holds only argon2id hashes.
- Anything under `infra/`.

---

## Orientation

- **Base branch.** PRs target `qa` (`.github/PULL_REQUEST_TEMPLATE.md`). This plan was written against `upstream/qa` at `175891e3`. Local `qa` was identical to it.
- **Backend.** Go `1.25.13`, `golang.org/x/crypto` already provides argon2id (`service/auth/password.go`). No new module.
- **Frontend.** Preact + zustand. PDF output reuses the dependency-free writer in `frontend/src/services/auth/recoveryCodeFileService.ts`. No new package.
- **Persistence.** Flat JSON under `DATA_DIR`, written with temp file + rename under an in-process mutex. There is no migration tooling. A missing file means "no invited credentials yet".
- **Checks.** `gofmt -l backend`, `cd backend && go vet ./... && go test ./...`, `go test ./...` from the repo root (catalog), `cd frontend && npm test && npm run build`. Commits use Conventional Commits with an area scope and `git commit -s`.

## Code this change builds on

| Concern | Existing code | How it is reused |
| --- | --- | --- |
| Password hashing | `service/auth/password.go` (`HashPassword`, `VerifyPassword`, argon2id, 12–1024 chars) | Unchanged. Used for temporary and permanent passwords. |
| Password login | `service/auth/local_admin_authenticator.go` (`login`, dummy hash for constant-time miss) and `Service.CompletePasswordLogin` (`service.go:283`) | Extended so `LoginLocal` (`service.go:235`) falls through to invited credentials after the local admin. |
| Short-lived pre-session state | `pendingLogin` + `signedPayload` (`service.go:33`, `signed_payload.go`), cookie `remote_2fa_pending` (`handlers/auth_cookie.go`) | Same pattern for a new `remote_password_change` cookie. |
| Session checks | `Service.CurrentSession` (`service.go:354`), `middleware/auth.go` `Wrap`, `AccessVerifier.Verify` (`access.go:31`) | No session is issued before the password change, so every gate already rejects. One extra check in `CurrentSession` adds defence in depth. |
| User directory | `service/user` (`Add`, `Remove`, roles), `UsersHandler` (`handlers/users_handler.go`) | `Add` stays the single place users are created. The handler's POST gains a sign-in-method field. |
| Identity cleanup | `service/user_removal_cleanup.go` | Also deletes the invited credential. |
| Email | `Services.Mailer` (`service/email/mailer.go`), builder `Required()` / `Send` (`builder.go:101`, `:288`), `Mailer.Enabled` | Invitation mail is `Required()` and sent synchronously, so the handler knows whether it was delivered. |
| Download as PDF | `recoveryCodeFileService.ts` (hand-written PDF) + `platform/fileDownloadService` | The PDF writer is moved into a shared helper and used by both files. |

## Design

### Decisions

1. **No session until the password is changed.** Signing in with a temporary password does not issue `remote_session`. The response is `{"passwordChangeRequired": true}` and sets an HttpOnly, 15-minute `remote_password_change` cookie. The cookie is signed with the session key and holds `{email, credentialVersion, exp}`. `/api/*`, `/ws*`, `/auth/verify` and the application gateway see no session, so they reject the user. No route list has to be maintained.
2. **Defence in depth in `CurrentSession`.** If the email has an invited credential with `MustChangePassword=true`, a password-method session for it is rejected. A cached in-memory set keeps this lookup off the hot path for everyone else.
3. **One credential store for invited accounts, kept apart from `local-admin.json`.** Use a new `DATA_DIR/account-credentials.json`, a map from email to `{passwordHash, mustChangePassword, temporaryExpiresAt, version, updatedAt}`. The single-owner rules for the local admin are left alone.
4. **The temporary password exists in plaintext once.** It is generated in the service, hashed, emailed (if SMTP is on) and returned once in the HTTP response. It is never stored, logged, or put in an error. 20 characters from a 32-symbol alphabet with no look-alikes (`ABCDEFGHJKMNPQRSTUVWXYZ23456789` + `-` grouping) gives about 100 bits. The alphabet contains no PDF-special characters.
5. **Temporary passwords expire** after 7 days (`AuthOptions.TemporaryPasswordTTL`). After that, login fails with the generic invalid-credentials error. The admin can issue a new one.
6. **"View credentials later" means re-issue, not recall.** The issue asks the admin to be able to "retrieve the credentials when requested without requiring another invitation". It also forbids plaintext persistence. Both cannot hold literally. The plan:
   - After an SMTP invite, the dialog asks "Also view or download the credentials?". The plaintext stays in that dialog's memory until it closes, so the admin can view or download it without sending a new invitation.
   - After the dialog closes, **Reset temporary password** on the user row creates a new password. It bumps `version`, which invalidates the old password and any pending change cookie, and shows the new password the same way. It does not re-add the user or change the role. It sends mail only if the admin ticks "email it".
   - *Rejected:* keeping the plaintext encrypted server-side for N minutes. It adds a key-management problem and a second copy of a secret for little gain.
7. **Delivery outcome is explicit.** `POST` returns `delivery: "sent" | "not_configured" | "failed"` with a safe `deliveryError`. The account is created in all three cases. The UI never says "emailed" unless the value is `sent`.
8. **Google stays an option.** A Google login for an invited-password account works as it does today, because Google proves the email. Password login keeps working too. An invited account created with `method: "google"` keeps today's behaviour exactly and still requires Google OAuth to be configured. `method: "password"` does not need Google.

### Flows

**Create (admin):** `POST /api/admin/users` `{email, role, method: "password"}`
1. Handler: `requireAdmin`, validate input.
2. `InvitationService.Invite` (new, `service/invitation`):
   1. `users.Add(email, role, caller)`. A duplicate returns `ErrUserExists` → 409 and no credential is written.
   2. Generate the password, then `auth.SetTemporaryPassword(email, plain)`. That hashes it and stores `{mustChange: true, expires, version+1}`. On failure it calls `users.Remove` (compensation) and returns 500. Nothing reports success.
   3. If `mailer.Enabled`: `Mail().To(email).Required()…Send(ctx)` with a 20s context. The mail holds the login URL (`auth.BaseURL()`), the email, the temporary password in a `Code` block, the expiry, and first-login steps. An error gives `delivery=failed`. The error is reduced to a fixed message such as "SMTP server rejected the message" and never includes the body.
   4. Return `{user, delivery, temporaryPassword, expiresAt}`.
3. The response sets `Cache-Control: no-store`.

**First login (invited user):**
1. `POST /auth/local/login` → `CompletePasswordLogin` → `LoginLocal` checks the local admin first, then invited credentials. Every path runs the same argon2 cost, using the dummy hash on a miss.
2. If the credential is `mustChange` and not expired: set the `remote_password_change` cookie and return `{passwordChangeRequired: true}`.
3. The SPA shows the "Set a new password" step, built the same way as `TwoFactorChallengeStep`.
4. `POST /auth/local/password/initial` `{newPassword}`. It is rate-limited with `localLoginLimiter` and needs the change cookie. The service checks the cookie signature, expiry, and that the cookie's `version` still matches the stored one. It also checks that the new password is not the temporary password, then `HashPassword` (≥12 chars). It saves `{mustChange: false, version+1}` and clears the cookie.
5. It then goes through `completeLogin(user, SignInMethodPassword)`, so 2FA, session registry and history behave as for any other login. It sets `remote_session` and returns `Status`.

**Removal:** `userRemovalCleanup` deletes the credential, so a re-invited email starts clean.

### API summary

| Method & path | Auth | Body → Response |
| --- | --- | --- |
| `POST /api/admin/users` | admin | `{email, role, method?: "google"\|"password"}`. If `method` is missing, it is `"google"` when Google is configured, else `"password"`. `"google"` → `User` (unchanged). `"password"` → `{user, delivery, deliveryError?, temporaryPassword, expiresAt}` |
| `POST /api/admin/users/{email}/temporary-password` | admin | `{sendEmail: bool}` → same shape. Refused (409) for the local admin and for Google-only users with no credential. |
| `GET /api/admin/users` | admin | `User` gains `signInMethods: ["google"\|"password"]` and `passwordChangePending: bool` |
| `POST /auth/local/login` | public | adds the `{passwordChangeRequired: true}` outcome |
| `POST /auth/local/password/initial` | change cookie | `{newPassword}` → `Status`, `{twoFactorRequired: true}`, or 400/401/429 |
| `GET /api/admin/email` | admin | already exists. The invite form uses `configured` to choose its wording. |

### Security checklist

- The plaintext password appears only in the service's local variables, the mail body, and the single HTTP response. Tests check it is absent from logs, `account-credentials.json`, `users.json`, and every error string.
- `Cache-Control: no-store` on responses that carry a password. The SPA holds it in component state only, never in zustand persistence, localStorage, or the URL.
- Login timing is equal for every combination of unknown email, local admin, invited user, wrong password and expired temp password. The dummy-hash pattern is extended, and the expiry check runs after the hash check.
- Rate limiting covers the change endpoint. The change cookie is `HttpOnly; Secure; SameSite=Strict`, signed and versioned, so it is single-use in practice.
- The invite route keeps `requireAdmin`. The middleware gate is not authorization.
- A duplicate email, a credential-store failure (with rollback), and an SMTP failure (account kept, reported as not sent) each have a test.

## Steps

Each backend step finishes with `gofmt -l backend` returning nothing and `cd backend && go vet ./... && go test ./internal/...`.

1. **Auth model and port.** Files: `service/auth/model.go`, `ports.go`.
   Add `AccountCredential{Email, PasswordHash string; MustChangePassword bool; TemporaryExpiresAt int64; Version uint64; UpdatedAt int64}`, an `AccountCredentialStore` port (`Get` returns `(nil, nil)` when absent; `Save`; `Delete`; `List`), and errors `ErrPasswordChangeRequired`, `ErrInvalidPasswordChange`, `ErrPasswordReuse`. Add `SignInMethod` values only if history must tell them apart. The default is to reuse `password`.
   Verify: `go build ./...`.

2. **File store.** Files: `stores/fileaccountcredentials/store.go` + `_test.go` (new), `stores/stores.go`, `service/services.go` (`Dependencies`), `cmd/remote/main.go`.
   Write `account-credentials.json` at mode 0600 with the temp+rename pattern from `fileauth.writeJSONLocked`, behind a mutex, with emails normalized. Wire it through all three composition literals.
   Verify: round-trip, `(nil,nil)` on absence, idempotent delete, `Perm()==0o600`, and `grep -n AccountCredentials` hits `stores.go`, `services.go` and `main.go`.

3. **Temporary password generator.** File: `service/auth/temporary_password.go` + test.
   `GenerateTemporaryPassword() (string, error)` uses `crypto/rand` with rejection sampling and outputs `XXXXX-XXXXX-XXXXX-XXXXX`.
   Verify: length, alphabet, `HashPassword` accepts it, and 10k samples give no duplicates.

4. **Invited-account authenticator.** File: `service/auth/account_password_authenticator.go` + test. `service.go` gains a field, wiring in `New`, and an `Options.TemporaryPasswordTTL` with validation.
   Methods: `setTemporary(email, plain)`, `login(email, password) (User, mustChange bool, err)`, `completeChange(email, version, newPassword)`, `delete(email)`. Add a cached `mustChange` set loaded at startup.
   `LoginLocal` order: local admin, then invited credential, then dummy. The `User.Sub` for invited password users is `"password"`.
   Verify: correct, wrong and expired passwords, wrong email, and constant work (the dummy hash runs on a miss, checked through a counting hasher seam in the test).

5. **Login and password-change service flow.** Files: `service/auth/service.go`, `password_change.go` + test.
   `CompletePasswordLogin` returns `LoginResult{PasswordChangeRequired: true, PendingToken}` when `mustChange` is set. `CompleteInitialPasswordChange(ctx, token, newPassword, ip, ua)` verifies the token, checks the version, rejects reuse, saves, then calls `completeLogin`. `CurrentSession` adds the step-2 defence check from *Decisions*.
   Verify: a pending token can't be replayed after a change or after a reset (version bump); an expired token gives `ErrInvalidPasswordChange`; 2FA-enabled users still get the 2FA step; a forged password-method session for a `mustChange` account is rejected by `CurrentSession`.

6. **Invitation service.** Files: `service/invitation/service.go`, `model.go`, `service_test.go` (new) and `service/services.go`.
   Depends on narrow ports: user adder/remover, credential setter, mailer (`Enabled`, `Mail`), base URL. `Invite` and `ResetTemporaryPassword` follow *Flows*. `userRemovalCleanup` gains `credentials.Delete`.
   Verify, with fakes: duplicate → `ErrUserExists` and no credential; credential failure → user removed; SMTP disabled → `not_configured` with the password returned; SMTP error → `failed`, account exists, error has no password; success → `sent`; the mail body contains the URL, email and password; a log-capture test finds no password in any log line.

7. **HTTP.** Files: `handlers/users_handler.go`, `auth_local_handler.go`, `auth_cookie.go`, tests; route registration in `auth_handler.go` and `transport.go`.
   - The users POST takes `method`. The `GoogleOAuthEnabled` 409 (`users_handler.go:53`) now applies only to `method=google`. Add the `/temporary-password` subroute. Set `no-store` on password responses. `List` gains `signInMethods` and `passwordChangePending`.
   - `/auth/local/login` handles the new result. `/auth/local/password/initial` is new, and a `cancel` route clears the cookie.
   Verify: 401/403 for non-admins; 409 duplicate; 201 with each `delivery` value; with a change cookie but no session, `/api/projects`, `/ws/...` and `/auth/verify` all give 401; a successful change sets `remote_session` and `/auth/me` reports authenticated with the right `isAdmin`.

8. **Frontend: API + state.** Files: `config/routes.ts`, `api/authApi.ts`, `api/userApi.ts`, `models/auth.ts`, `state/hooks/auth/useLocalAuthController.ts` + `*.test.ts`.
   Add the `passwordChangeRequired` login outcome, `submitInitialPassword`, the invite/reset calls, and types.
   Verify: `npm test` with new pure-state tests (form rules: ≥12 chars, confirmation match, not equal to the temp password).

9. **Frontend: first-login UI.** Files: `ui/auth/PasswordChangeStep.tsx` (new), `ui/auth/LoginScreen.tsx`.
   After `passwordChangeRequired`, render the change step, which has no way to skip it. Change the login copy "Invited users sign in with Google." so it covers both methods. Show the password form even when Google is configured.
   Verify: `npm run build`. Manual QA in criteria 6–8.

10. **Frontend: admin UI + PDF.** Files: `ui/account/UsersPanel.tsx` (+ `InviteCredentialsDialog.tsx`), `services/files/simplePdf.ts` (extracted from `recoveryCodeFileService.ts`), `services/auth/invitationCredentialFileService.ts` + tests.
    - The add form gets a sign-in method choice. Google is offered only if configured; password is the default otherwise. It shows a note on whether SMTP is configured.
    - Result dialog:
      - `sent`: "Invitation emailed" plus "View / download credentials?"
      - `not_configured`: the credentials are shown at once with a warning: "Not emailed: SMTP is not configured".
      - `failed`: the credentials are shown with "Email could not be sent".
      - All three have Copy, Download PDF, and Done. Done clears the state.
    - User rows show "Password change pending" and a **Reset temporary password** action.
    - The PDF holds the product name, login URL, email, temporary password (monospaced), expiry, and first-login steps.
    Verify: `recoveryCodeFileService.test.ts` still passes unchanged after the extraction; the new PDF test checks the header, `%%EOF`, a valid xref, and that the content stream contains the email and password; `npm run build`.

11. **Docs.** Files: `ARCHITECTURE.md` ("Platform identity": invited accounts may use email + password; `account-credentials.json`), `docs/02-workspaces/02-auth-users-and-access.md`, `docs/03-platform/08-api-and-realtime.md` (new routes), `docs/threat-model.md` (new file, temp password in email, cookie), `README.md` user guide.
    Verify: `grep -rn account-credentials.json ARCHITECTURE.md docs/` shows each file updated.

12. **Full verification + PR.** Run every check in *Orientation*. Commit per step (`feat(auth): …`, `feat(users): …`, `docs(auth): …`) with `-s`. Update this PR's description with test output, and add screenshots of the three dialog states and the change step.

## Data / migrations

- New file `DATA_DIR/account-credentials.json`, mode 0600. When it is absent, there are no invited password accounts. Existing installs need no migration.
- `users.json` is unchanged. Sign-in methods are derived from it and the credential file.
- Rollback: an older binary ignores the new file. Invited password users can't sign in under it, but Google users and the local admin are unaffected.

## Risks & open questions

| # | Item | Mitigation / proposal |
| --- | --- | --- |
| R1 | Emailing a password is weaker than a magic link. | The issue requires it. The password is temporary, expires in 7 days, and the user must change it before any session. A magic-link flow could be a follow-up issue. |
| R2 | An attacker reading the mailbox before the user does. | Same as R1. A reset invalidates the old password. |
| R3 | The SMTP send blocks the request for up to 20 seconds. | A bounded context plus a spinner in the dialog. The account is already created, so a timeout is reported as `failed`, not lost. |
| Q1 | Is "retrieve credentials later" satisfied by an in-dialog view + **Reset temporary password**, as in *Decisions 6*? | **Needs the issue owner's confirmation.** This is the only design that keeps "never persisted in plaintext". |
| Q2 | Should a `mustChange` account be allowed to finish a Google login before setting a password? | Proposed: yes, because Google proves ownership. The temporary password stays valid until it expires or is changed. Alternative: Google login also redirects to the change step. |
| Q3 | 7-day expiry? | Proposed default, set in `AuthOptions` and not shown in the UI. |

## Acceptance criteria (QA)

1. Admin invites `a@example.com` as member with SMTP configured → the email arrives with the URL, email, password and instructions; the UI says "emailed".
2. Same with SMTP not configured → the credentials are shown at once with the not-emailed warning; the PDF downloads and opens.
3. Same with SMTP configured but broken (wrong app password) → the account exists and the UI says the email was **not** sent and shows the credentials.
4. Inviting an existing email → 409, no change to the existing user, no email.
5. `grep -rF "$TEMP_PW" /opt/remote.futrx/data /var/log journalctl -u remote.futrx` → no matches.
6. Invited user logs in with the temp password → change step shown; `curl` to `/api/projects` and a preview host with the browser's cookies → 401.
7. The new password must be ≥12 chars and not the temp password → after the change the user lands in the workspace with member rights (no admin settings).
8. A second login with the temp password fails; the new password works.
9. Reset temporary password → the old temp password and an open change step both stop working; the new one works.
10. Removing the user and re-inviting gives a fresh credential.
11. Google-only invite and the local-admin login behave exactly as before.
12. An expired temp password (set the TTL low on QA) is rejected with the generic message.

## Progress

- [ ] 1–12 (not started)

## Outcome

<!-- filled after implementation -->
