# Per-container code-server

## Problem

Today the platform runs a single code-server as **host root** at
`code.<host>`. This has three sharp edges:

1. **RAM stacks.** code-server keeps a VS Code server process alive per
   connected workspace, and its default reconnection grace window is ~3h.
   On a busy box every project the operator opens leaves a server resident
   long after the tab is closed, so memory creeps up and never comes back
   down until the host instance is restarted.
2. **Wrong-uid git objects.** Because the host instance runs as uid 0 on the
   *host*, any `git` operation it performs against a project's bind-mounted
   workspace writes objects owned by host-root. Project containers are
   idmapped (container-root is a high host uid), so those uid-0 objects land
   inside the idmapped mount as an unmapped owner and the container's own
   git refuses to touch them ("dubious ownership" / EPERM on pack files).
3. **One blast radius.** A single shared IDE means one process with read/write
   reach into *every* project's files at once.

## Design (Option A: per-container code-server)

Each project LXD container runs **its own** code-server, installed into the
base image and reachable at `<slug>.code.<host>`. Inside the container the
stack is fully socket-activated and scales to zero:

- `code-server.socket` listens on `0.0.0.0:8080` (cheap; always armed).
- `code-server-proxy.service` is `systemd-socket-proxyd` with
  `--exit-idle-time=20min`, forwarding to `127.0.0.1:8081`. It `Requires=`
  the real service, so the first connection pulls code-server up.
- `code-server.service` is code-server itself, listening on loopback
  `127.0.0.1:8081`, with `StopWhenUnneeded=yes` so it stops the moment the
  proxy idle-exits.

Net effect: a container with nobody connected runs only a listening socket
(near-zero RAM); 20 minutes after the last IDE tab disconnects, the whole
stack is gone. code-server runs as **container** root against the workspace,
so git objects are written with the uid the container expects — fixing (2).
Each project is isolated — fixing (3).

### Routing

`<slug>.code.<host>` is a new wildcard Caddy site (`*.code.${HOSTNAME}`). It
sits behind the **same Google admin gate** (`forward_auth` →
`/auth/verify`) as the existing `code.${HOSTNAME}` block, strips the
platform's admin-session cookies before the request enters the container,
and reverse-proxies to `<slug>.lxd:8080`. On-demand TLS is gated by the
backend's `/internal/tls-ask`, now extended to also accept
`<slug>.code.<host>` for real projects. `auth: none` inside the container is
safe because :8080 is the only reachable port and Caddy gates it.

## Files changed

- `backend/internal/manager/containers/templates/code-server-up.sh` — new;
  idempotent in-container install recipe (deb install, config, three systemd
  units, settings, pinned extensions).
- `backend/internal/manager/containers/code_server.go` — new;
  `EnsureCodeServer` (migration path for pre-image containers), mirrors the
  container migration helper pattern.
- `backend/internal/manager/containers/baseimage.go` — bakes the recipe into
  the base image after the browser-GUI layer.
- `backend/internal/manager/containers/lifecycle.go` — best-effort
  `EnsureCodeServer` on every `Launch`.
- `backend/internal/transport/http/handlers/project_handler.go` — new
  `codeHostPattern`; `HandleTLSAsk` accepts the dev-URL or code host.
- `infra/templates/Caddyfile.tmpl` — new `*.code.${HOSTNAME}` block.

## Validated

- `go build ./...` and `go vet ./...`.
- `bash -n code-server-up.sh` (shell syntax).
- `caddy validate` on the rendered Caddyfile.
- `systemd-analyze verify` on the three units.

## NOT yet done (follow-ups)

- **Base-image rebuild + live smoke test.** The recipe is baked by
  `BuildBaseImage`, but the published image must actually be rebuilt and a
  real container exercised end-to-end (cold connect pulls code-server up,
  idle for 20 min tears it down).
- **UI flip.** The frontend still links to the host `code.<host>` instance.
  Pointing the UI at the per-container IDE means updating
  `frontend/.../ideLinks.ts` and the backend `chat_handler.go` IDE-open
  path, then **retiring the host code-server instance**. That is a separate
  change once this one is verified live.
