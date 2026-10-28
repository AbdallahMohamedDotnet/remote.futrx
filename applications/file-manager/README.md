# File Manager

Optional, project-scoped workspace browsing. Install **File Manager** from a
project's Applications tab; its Files drawer appears beside the chat only while
that installation is running. New projects do not inherit it.

## Package

| Capability | Owner |
|---|---|
| Container service | `backend/container/cmd/file-manager/` builds `/usr/local/bin/file-manager` |
| Lifecycle | `application.json` declares `remote-file-manager`, listening on port 8844 |
| Browser surface | `ui/scripts/main.js` registers the Files drawer |
| Files page | `backend/container/internal/presentation/web/assets/`, embedded in the service binary |
| Cleanup | `infra/uninstall.sh` removes the binary after Remote stops the service |

There is no host backend, custom install script, database, configuration input,
or agent skill. Remote builds the container program and manages the service.
Its authenticated web gateway exposes the service at the project's `files`
application hostname. No independent login or public host port is configured.
As with Terminal, the service relies on the gateway for user authentication and
project membership; its container listener is not an independent auth boundary.

## Container application layers

`backend/container/` is an independent Go application. Its generated module
contains only container source; it does not import the host backend or Remote's
internal packages. The binary name remains `file-manager`.

```text
backend/container/
  cmd/file-manager/main.go         command and dependency composition
  internal/
    config/                       workspace, server settings, resource limits
    model/files/application/      application results and errors
    port/files/inbound/           file operations exposed to presentation
    port/files/outbound/          filesystem and archive-spool contracts
    service/files/                listing/search policy and download workflows
    repository/workspace/         rooted filesystem, listing, search, ZIP I/O
    repository/archives/          bounded temporary files and admission slots
    presentation/http/            routes, DTOs, headers and error mapping
    presentation/web/assets/      embedded browser page
```

The command injects concrete repositories into the file service, then injects
that service into HTTP presentation. Services do not depend on HTTP or concrete
filesystem repositories. HTTP presentation maps application results into JSON
and streams completed downloads. Archive preparation, spooling, and failure
cleanup run behind the service contract. Each spool instance owns its limits
and concurrency slots; request handlers do not mutate package globals.

A host API application, when needed, is a separate composition at
`backend/main.go` with its own host packages, as in Code Server. Remote excludes
`backend/container/` from that host build. File Manager's existing behavior uses
its container web service directly and needs no host RPC process.

## Use

Open Files in the chat header. Expand folders, refresh the tree, or search
filenames with at least two characters. Dotfiles are included. Downloads support
individual files and folder ZIPs, including the workspace root through
`/api/archive`. Listings stop at 10,000 entries; searches stop at 300 matches or
200,000 visited entries and report truncation.

File clicks use the shared `remote.files.open` API: supported media opens in
Remote's media viewer, source/text files use an installed application opener
(such as Code Server), and other files download. Code Server is optional.
The iframe bridge checks both the sender window and exact application origin;
project and chat context come from the drawer, not from iframe messages.

All file operations stay inside `/workspace`, including symlink resolution and
rooted final opens. ZIPs skip special files and escaping symlinks, honor request
cancellation, and bound source bytes, entry count, compressed spool size (1 GiB),
and concurrency (two archives). Temporary archives are deleted after download.

## Lifecycle and verification

Stop removes the drawer and stops the service; start restores it. Uninstall
removes the application binary and Remote-owned service files, preserving all
workspace files. Reinstall builds from the current package. When changing the
application, bump its manifest version so installed copies reconverge.

Run from the repository root (the optional browser check needs Node, Playwright,
and its Chromium installation):

```bash
go test ./applications/file-manager/backend/container/...
go vet ./applications/file-manager/backend/container/...
node --test applications/file-manager/ui/scripts/*.test.mjs
FUTRX_BROWSER_TEST=1 go test ./applications/file-manager/backend/container/... -run TestFileManagerBrowser -v
```

After a real project installation, check the Files drawer, search, nested folder
ZIPs, media previews, and Code Server opening. Confirm another project has no
Files action until installed. Check stop/start/uninstall and container recovery.
Inside the target container, verify `systemctl status remote-file-manager` and
`curl -fsS http://127.0.0.1:8844/health`. Container provisioning and authenticated
web-gateway routing need a live Remote/LXD installation; unit tests do not prove
those paths.
