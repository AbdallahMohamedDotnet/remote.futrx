# Project application web routes

## Reaching the application's HTTP server

Declare `"web": { "port": 8400 }`, scope `["project"]`, and a service
listening on `0.0.0.0:8400`. A signed-in project member opens:

```text
https://remote.example/apps/my-project/editor/src/main.ts?line=12
    -> http://my-project.lxd:8400/src/main.ts?line=12
```

The handler looks up the slug among the caller's visible projects, looks up
the application in the catalog, and requires a `running` project instance.
The manifest supplies the upstream port. A URL cannot select an arbitrary port
or upstream host. The route does not start a stopped installation or recreate a
project container. The declared service must already be reachable.

The proxy strips the `/apps/<slug>/<application-id>` prefix, preserves the
query, removes all request cookies and upstream `Set-Cookie` headers, rewrites
root-relative redirects into the application prefix, and restricts an upstream
`Service-Worker-Allowed` header to that prefix. The handler uses Go's reverse proxy, including its HTTP upgrade handling; a
live WebSocket lifecycle still needs runtime verification.

Use relative asset URLs or explicitly account for the prefix in the app.
Remote does not rewrite HTML, JavaScript, absolute redirects, or arbitrary
application URL generation. Applications that depend on their own browser
cookies are not compatible with this gateway's cookie stripping.

All catalog-declared `web.port` values are excluded from public preview
sharing, even if that app is not installed in a particular project. The
callback reads the current catalog on each check, so package changes affect
new grants and validation of existing grants. This is a catalog-wide port
reservation, not a firewall or a restriction on authenticated previews.

See [12 — HTTP API](12-http-api.md) for status
codes. Read [13 — Security model](13-security-model.md)
before using this for project-controlled content: this route shares the main
browser origin, and the LXD bridge bypass remains a separate boundary.

`web.port` must be 1024–65535. Remote requires a declared service and exactly
project scope. No host port is allocated; without `port.internal`,
`APP_INTERNAL_PORT` is zero and `healthcheck.command` must be omitted.

The route returns 404 for invisible projects, absent catalog web declarations,
or stopped/missing installations; unavailable upstreams produce 502.
Bare application URLs gain a trailing slash (308). Upstream keep-alives are
disabled. No response HTML or arbitrary URL rewriting is performed.

## Source and verification

- [backend/internal/integration/containers/applications/registry_clone.go](../../../backend/internal/integration/containers/applications/registry_clone.go)
- [backend/internal/integration/containers/applications/registry_manifest.go](../../../backend/internal/integration/containers/applications/registry_manifest.go)
- [backend/internal/integration/containers/applications/registry_validation.go](../../../backend/internal/integration/containers/applications/registry_validation.go)
- [backend/internal/integration/containers/applications/registry_web_test.go](../../../backend/internal/integration/containers/applications/registry_web_test.go)
- [backend/internal/service/applications/model.go](../../../backend/internal/service/applications/model.go)
- [backend/internal/service/applications/service.go](../../../backend/internal/service/applications/service.go)
- [backend/internal/service/applications/web_test.go](../../../backend/internal/service/applications/web_test.go)
- [backend/internal/service/services.go](../../../backend/internal/service/services.go)
- [backend/internal/service/share/errors.go](../../../backend/internal/service/share/errors.go)
- [backend/internal/service/share/port_policy_test.go](../../../backend/internal/service/share/port_policy_test.go)
- [backend/internal/service/share/service.go](../../../backend/internal/service/share/service.go)
- [backend/internal/transport/http/handlers/applications_handler.go](../../../backend/internal/transport/http/handlers/applications_handler.go)
- [backend/internal/transport/http/handlers/applications_web_handler.go](../../../backend/internal/transport/http/handlers/applications_web_handler.go)
- [backend/internal/transport/http/handlers/applications_web_handler_test.go](../../../backend/internal/transport/http/handlers/applications_web_handler_test.go)
- [backend/internal/transport/http/handlers/auth_verify_handler.go](../../../backend/internal/transport/http/handlers/auth_verify_handler.go)
- [backend/internal/transport/http/handlers/auth_verify_share.go](../../../backend/internal/transport/http/handlers/auth_verify_share.go)
- [backend/internal/transport/http/handlers/auth_verify_share_test.go](../../../backend/internal/transport/http/handlers/auth_verify_share_test.go)
- [frontend/src/models/application.ts](../../../frontend/src/models/application.ts)

The included unit tests verify decisions and generated requests/commands.
No live container installation, authenticated browser flow, or QA deployment
was performed for this split.

## What to verify

Tests cover catalog service/scope/port validation, running-instance lookup,
dynamic public-share port protection, path/query/cookie/redirect/worker-header
proxy transformations and malformed routes. Verify real assets, redirects and
WebSockets; test nonmember access and a stopped install in a live project.

### Responsibility boundaries

- [applications_web_proxy.go](../../../backend/internal/transport/http/handlers/applications_web_proxy.go) — Owns upstream request rewriting, cookie filtering, redirects, and service-worker scope; the handler retains caller/project authorization.
