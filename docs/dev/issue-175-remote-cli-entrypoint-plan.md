# Issue #175: Remote CLI entry point implementation plan

## Scope and investigation state

This plan addresses [GitHub issue #175](https://github.com/futrx-com/remote.futrx/issues/175), where the documented `remote setup-token` recovery command fails with `remote: command not found` on an installed Ubuntu server.

The investigation was performed against canonical `upstream/qa` at commit `fecf33be93abc54f008e60521cd1033f50c8c6a3`. At investigation time, the local worktree was detached at `e7dfbcff`, the local `qa` branch was 347 commits behind `upstream/qa`, and `.gitignore` contained a pre-existing user modification. No implementation files, branches, commits, pushes, or pull requests were created during the investigation.

The attached code-refactorer guidance was applied where relevant, particularly its backend/configuration ownership, lifecycle, security, and verification rules. This work is a focused bug fix, not an authorization for unrelated restructuring.

## Confirmed diagnosis

The defect has two linked causes.

### No operator-facing command is installed

- `infra/steps/02-app.sh:37-44` builds the executable only at `${INSTALL_DIR}/backend/remote`.
- `infra/templates/remote.futrx.service.tmpl:6` launches that private binary path directly.
- `infra/templates/remote-futrx-host-clis.sh.tmpl` exposes managed agent CLIs under `${INSTALL_DIR}/data/host-clis/bin`, not the Remote backend.
- No installer, convergence step, or migration creates `/usr/local/bin/remote` or another `remote` entry point on a normal server `PATH`.
- `README.md:325-330` and `docs/02-user-guide/12-troubleshooting.md:9` nevertheless instruct operators to run `remote setup-token`.

### A symlink alone would use incorrect runtime configuration

- `backend/cmd/remote/main.go:47-53` calls `config.Load()` before dispatching CLI commands.
- `backend/cmd/remote/cli.go:18-25` passes `cfg.DataDir`, `cfg.BaseURL`, and the configured setup-token TTL into the command.
- `backend/internal/config/config.go:78-99` defaults to `/opt/remote.futrx`, `/opt/remote.futrx/data`, and an empty `BASE_URL` when the environment is absent.
- `backend/cmd/remote/setup_token.go:28-31` rejects an invalid or empty base URL.
- The correct `BASE_URL`, `DATA_DIR`, and `INSTALL_DIR` currently exist only in the rendered systemd unit at `infra/templates/remote.futrx.service.tmpl:12-14`.

Consequently, the current installation fails first with `command not found`. Adding only a symlink would then fail because `BASE_URL` is empty. A custom `FUTRX_INSTALL_DIR` installation would additionally target the wrong installation and data directories.

The backend command itself is already correctly separated from server startup. CLI dispatch returns from `main` before stores, services, background work, transport, or `ListenAndServe` are initialized. Token rotation is also already correct: `backend/internal/stores/fileauth/store.go:266-285` overwrites the previous record and persists only its hash, with permission coverage in `store_test.go`.

## Impact by deployment path

### Fresh installations

The installer builds and starts the backend but does not create a shell command. Setup-token recovery therefore fails exactly as reported.

### Existing installations

Existing servers need a full infrastructure convergence to gain the host entry point and shared configuration. Migration must recover the hostname from the old unit before replacing it.

### Custom `FUTRX_INSTALL_DIR` installations

Direct binary defaults point to `/opt/remote.futrx`, so a host entry point must carry the actual installation directory and derived data directory.

### Full infrastructure convergence

`infra/update.sh` invokes `infra/install.sh`, so this is the correct path to create or repair the host entry point, canonical configuration, ownership, permissions, and service unit.

### Application-only updates

`infra/deploy-app.sh` deliberately replaces only `${INSTALL_DIR}/backend/remote` and does not converge host configuration. Once installed, a stable launcher will automatically execute the replaced binary. An old server without the launcher must first receive the infrastructure-bearing release.

## Approaches considered

### Install a symlink to `backend/remote`

This is small and naturally forwards arguments and exit status, but it does not provide `BASE_URL`, `DATA_DIR`, or `INSTALL_DIR`. It fails custom installations and does not meet the acceptance criteria.

### Install a wrapper with duplicated hard-coded configuration

This could work functionally, but systemd and the shell wrapper would have independently editable copies of the same runtime configuration. Repairing one would not necessarily repair the other, violating the no-drift requirement.

### Parse the systemd unit from the backend CLI

This would make systemd nominally authoritative, but it couples application code to unit names and systemd quoting rules. It is fragile, harder to test, and adds systemd access to a command that otherwise only needs file-backed authentication state.

### Shared root-managed configuration and common launcher

This is the recommended approach:

- Install `/etc/default/remote.futrx` as the canonical source of `BASE_URL`, `DATA_DIR`, and `INSTALL_DIR`.
- Install `/usr/local/bin/remote` as a root-owned launcher.
- Have the launcher read and export the canonical configuration, change to the configured installation directory, and run `exec "$INSTALL_DIR/backend/remote" "$@"`.
- Have `remote.futrx.service` invoke the same absolute launcher.

Both interactive shells and systemd then use the same binary-resolution and configuration path. `exec` replaces the shell with the Go process, so there is no lingering wrapper process and systemd continues to supervise the backend directly after launch.

Trade-offs:

- Service startup gains one short shell execution before `exec`.
- Future infrastructure updates must preserve the launcher/configuration contract.
- `update.sh` must read the hostname from the canonical configuration while retaining old-unit fallback during migration.
- Repository guidance classifies this as an infrastructure change, requiring a major/minor release rather than an application-only patch release.

## Expected files

New files:

- `infra/templates/remote-cli.sh.tmpl`
- `infra/templates/remote.futrx.env.tmpl`
- `infra/tests/remote-cli-entrypoint-test.sh`

Modified files:

- `infra/install.sh`
- `infra/steps/04-backend-svc.sh`
- `infra/templates/remote.futrx.service.tmpl`
- `infra/lib/install-migration.sh`
- `infra/update.sh`
- `infra/tests/install-migration-test.sh`
- `infra/tests/host-agent-install-test.sh`
- `infra/qa/install.sh`
- `infra/qa/update.sh`
- `.github/workflows/ci.yml`
- `README.md`
- `docs/02-user-guide/12-troubleshooting.md`
- `docs/04-operations/09-deployment-and-operations.md`
- `docs/known-limitations.md`
- `backend/cmd/remote/setup_announce.go`
- `backend/internal/service/auth/setup_token.go`

No business-service, persistence-model, frontend, or application-deployer production changes are expected.

## Implementation steps

1. Add a canonical environment template containing the installation-specific `BASE_URL`, `DATA_DIR`, and `INSTALL_DIR`.
2. Add a launcher template that loads only that configuration, requires all three values, avoids `eval`, quotes paths and arguments, changes to the configured installation directory, and invokes the backend with `exec`.
3. Extend the installer's controlled template substitutions with the canonical config and launcher paths.
4. Update backend-service convergence to render into temporary files, syntax-check the launcher, and install the managed artifacts securely.
5. Install `/etc/default/remote.futrx` as `root:root` with mode `0644`.
6. Install `/usr/local/bin/remote` as `root:root` with mode `0755`, replacing a missing, stale, incorrectly linked, or incorrectly permissioned entry.
7. Change `remote.futrx.service` to invoke `/usr/local/bin/remote`. Keep the existing service name, port, PATH, HOME, `KillMode`, restart policy, and health checks.
8. Remove duplicated installation-specific service configuration so the common launcher/configuration remains authoritative.
9. Update hostname recovery to prefer `BASE_URL` from `/etc/default/remote.futrx`, then fall back to the old canonical or legacy unit.
10. Leave `infra/deploy-app.sh` unchanged; it will continue replacing the backend binary behind the stable launcher.
11. Change operator-facing instructions and messages to use `sudo remote setup-token`, because installation data is root-owned.
12. Add focused infrastructure, migration, QA, and regression coverage.

## Migration behavior

### Fresh installation

The installer creates the canonical configuration and launcher, starts systemd through the launcher, and verifies backend health. `command -v remote` succeeds immediately.

### Existing installation

The full updater first recovers the hostname from the old systemd unit. Normal installer convergence then creates the canonical configuration and launcher, replaces the unit, restarts the service, and verifies health.

### Future convergence and repair

Future updates read the hostname from the canonical configuration. Every full convergence re-renders and repairs the config, launcher, owner, mode, and unit. Old-unit parsing remains as a migration fallback.

### Custom installation path

The canonical configuration records the exact overridden installation directory. Both service startup and interactive CLI execution resolve the corresponding backend binary and data directory.

### Application-only deployment

Application-only deployment continues to replace the binary only. It neither creates nor repairs host infrastructure. A server predating this fix must use the full infrastructure update once.

## Security and backward compatibility

- The launcher and configuration are writable only by root.
- The configuration contains public routing and filesystem paths, not credentials; mode `0644` permits use from a normal shell without permitting modification.
- Installer-managed values replace incoming `BASE_URL`, `DATA_DIR`, and `INSTALL_DIR` so callers cannot accidentally redirect the privileged command.
- The launcher will not use `eval`.
- `"$@"` preserves argument boundaries; `exec` preserves output, errors, signals, and exit status.
- `remote setup-token` returns through CLI dispatch before any HTTP listener or backend lifecycle starts.
- Token plaintext remains terminal-only; only its hash is persisted.
- Reissuing overwrites the record and invalidates the previous token.
- Existing service semantics and health checks remain in place.
- Old service units remain readable during migration.
- No web endpoint or remotely accessible token-minting path is introduced.

## Tests

### Focused infrastructure tests

The new entrypoint test will cover:

- Default `/opt/remote.futrx` rendering.
- An overridden installation directory.
- Selection of the exact installed backend binary.
- Correct `BASE_URL`, `DATA_DIR`, and `INSTALL_DIR`.
- Argument forwarding, including whitespace and shell-sensitive characters.
- Stdout and stderr forwarding.
- Exit-code forwarding.
- Process replacement through `exec`.
- Clear failure for missing or incomplete configuration.
- Repair of a missing, stale, or incorrect entry point.
- Explicit secure ownership and permission installation.
- The shared launcher/configuration contract used by both shell and systemd.

Migration tests will cover new canonical configuration, pre-launcher units, legacy-unit fallback, and default/overridden paths.

### Existing backend coverage

Run the suites covering:

- Setup URL generation.
- Correct data-directory writes.
- Hash-only persistence.
- `0600` token-record permissions.
- Reissue rotation and rejection of the old token.
- Refusal after an administrator exists.
- CLI execution without creating a session key.
- CLI dispatch returning before server startup.

A narrow dispatch regression test will be added only if the infrastructure test cannot directly prove the early-return behavior.

### Fresh-install QA

Extend `infra/qa/install.sh` to verify:

- `command -v remote` succeeds.
- Launcher and configuration ownership and modes are correct.
- `sudo remote setup-token` succeeds.
- The printed URL uses `https://$QA_PUBLIC_HOST/?token=`.
- `setup-token.json` is written under the installed data directory.
- A second invocation changes the stored hash.
- The backend remains active and healthy.

### Full-convergence QA

Extend `infra/qa/update.sh` to remove or replace the managed launcher before convergence, run the full updater, and verify repair, permissions, configuration, service activity, local health, public health, and deployed SHA.

### Application-only regression

Confirm that the application deployer still replaces and rolls back only the backend binary, does not begin host convergence, and remains reachable through an already installed launcher.

### Required repository validation

- `gofmt -l .`
- `go vet ./...`
- `go test ./...`
- `go build ./...`
- `npm test`
- `npm run build`
- Every infrastructure script test in CI, including the new entrypoint test.
- Shell syntax checks for every changed or new shell file.
- Fresh QA installation on a rebuilt Ubuntu VM.
- Full `infra/qa/update.sh` against an existing installation.
- Focused application-only deployment regression.
- Final diff and status review for unrelated changes.

## Risks and rollback

Risks:

- Incorrect launcher/config rendering could prevent service restart.
- Hostname recovery could fail on a mixed old/new installation.
- Unusual custom paths could expose quoting assumptions.
- Patch-only updates cannot migrate old installations because they intentionally avoid host convergence.
- The QA repair test deliberately damages the test launcher before verifying convergence and must run only on the designated QA host.

Mitigations:

- Render and syntax-check before installation.
- Use quoted forwarding and a fixed root-owned configuration location.
- Preserve old-unit hostname fallback.
- Verify service health immediately after restart.
- Test default and overridden installation paths.
- Deliver through an infrastructure-bearing major/minor release.

Rollback procedure:

1. Restore the previous service template or release.
2. Run full convergence from the previous ref.
3. Verify the service is healthy through its previous direct binary path.
4. Only after service recovery, remove the unused launcher and canonical config.
5. Application-only rollback remains unchanged because it continues backing up and restoring the backend binary.

## Acceptance criteria

- `command -v remote` succeeds after a fresh installation.
- `sudo remote setup-token` prints a valid setup URL before an administrator exists.
- The URL uses the hostname configured during installation.
- The token is written to the correct data directory.
- Reissuing the token invalidates the previous token.
- Custom `FUTRX_INSTALL_DIR` installations work.
- Arguments, output, errors, signals, and exit codes are preserved.
- CLI mode exits without starting another backend server.
- Systemd and interactive CLI use one launcher and one configuration source.
- Full convergence repairs missing or stale launcher/configuration files.
- Launcher/configuration ownership and permissions are verified.
- Existing systemd startup and application health checks continue working.
- Application-only deployment continues replacing the backend without host convergence.
- Automated tests cover default and overridden installation paths.

## Remaining decisions

- Repository guidance requires an infrastructure change to ship in a major/minor release. Choosing the release number is outside this issue's implementation scope.
- The canonical repository is locally configured as `upstream`, while `origin` points to `AbdallahMohamedDotnet/remote.futrx`. Implementation should update and branch from `upstream/qa`, then push to the canonical repository so the PR is opened in `futrx-com/remote.futrx` with base `qa`. Remote names should not be changed merely to satisfy the wording “origin/qa.”
