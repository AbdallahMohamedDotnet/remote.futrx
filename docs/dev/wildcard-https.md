# Wildcard HTTPS

For `HOSTNAME=remote.example.com`, Remote configures Caddy to manage exactly two
names: `remote.example.com` and `*.remote.example.com`. Caddy obtains a separate
certificate for each and renews them automatically. Creating or deleting projects
and applications never requests another certificate. There is no TLS admission
endpoint, per-host issuer, certificate export script, or manually maintained PEM
file. Caddy persists its own keys, certificates, and renewal state in its normal
service storage; preserve that storage across host upgrades.

| URL | Route |
| --- | --- |
| `https://remote.example.com/` | Platform |
| `https://code--gamerhead.remote.example.com/?folder=%2Fworkspace` | Manifest label `code`, project slug `gamerhead` |
| `https://browser--gamerhead.remote.example.com/` | Manifest label `browser`, same project |
| `https://dev--gamerhead--3000.remote.example.com/` | Project preview on port 3000 |
| `https://app--abcdef123456--instance.remote.example.com/` | App without a manifest subdomain |
| `https://code.remote.example.com/gamerhead/` | Built-in editor launcher, until the Code Server migration removes it |

Each route adds exactly one DNS label to the **complete** configured hostname.
The two separators in preview/unnamed-app labels keep those namespaces distinct
from `<app-label>--<project-slug>`. App labels and new project names reject `--`.
Request authorization still checks project membership, or a valid public-preview
share. Preview ports must be 1024–65535; application ports come from the manifest.
A wildcard certificate authenticates the hostname, not the caller.

## Choose a DNS provider

Create DNS records for `remote.example.com` and `*.remote.example.com` pointing
at the ingress server. The provider must support automated DNS-01 challenges
through a [Caddy DNS provider module](https://caddyserver.com/docs/modules/).
Use the module's documentation for its required permissions and options. Remote
does not select a provider on the administrator's behalf.

Before running the installer/updater, create these two administrator-owned files
on the **Remote host**, outside the checkout:

1. `/etc/caddy/remote-dns.env`, owned by root, mode `0600`. Set the non-secret
   module name as `REMOTE_DNS_PROVIDER=<module-name>`, unquoted on its own line.
   Optionally set `REMOTE_DNS_PACKAGE=<Go-package-path>@<version>` to select a
   package/version. Without it, the package is `github.com/caddy-dns/<module-name>`.
   Add the provider's credential environment variables in `NAME=value` format.
   Values requiring quotes should use double quotes; do not use shell expansion.
2. `/etc/caddy/remote-dns.caddy`, owned by root and readable by the `caddy` service.
   Put the provider's `dns` directive and options here. Reference credentials with
   `{env.VARIABLE_NAME}` rather than writing tokens into the Caddyfile.

For example, **if the administrator chooses Cloudflare**, the files are:

```dotenv
# /etc/caddy/remote-dns.env (root:root, 0600)
REMOTE_DNS_PROVIDER=cloudflare
CLOUDFLARE_API_TOKEN=replace-with-your-zone-dns-token
```

```caddyfile
# /etc/caddy/remote-dns.caddy (root:root, 0644)
dns cloudflare {env.CLOUDFLARE_API_TOKEN}
```

Cloudflare is an example, not a default. Another provider may need a block with
multiple options or several credentials. The snippet accepts that provider's
native Caddy syntax. Providers with a package outside `github.com/caddy-dns/`
set `REMOTE_DNS_PACKAGE` explicitly. Package paths and module names are public
configuration; credential values are never printed by the installer.

The installer copies Caddy to a staging directory, adds the chosen module using
[Caddy's package command](https://caddyserver.com/docs/command-line#caddy-add-package),
and validates the rendered configuration before replacing live files. That command
uses the latest Caddy release and optionally the specified plugin version; it needs
outbound access to Caddy's build service. Subsequent updates reuse the managed
binary when it already contains the selected module/package and requested version.
Set a newer `REMOTE_DNS_PACKAGE` version to update the plugin and Caddy. Unpinned
packages are resolved when first added, then reused. This requires the official Caddy package from
the installer, whose CLI includes `add-package`.

The managed binary lives at `/usr/local/lib/remote-caddy/caddy`; a systemd drop-in
selects it without overwriting the distribution's `/usr/bin/caddy`. The drop-in
loads the environment file for Caddy startup and reload. Credential changes
trigger a restart; configuration-only changes reload. Missing provider setup,
build failures, and invalid candidate configuration stop the update before
replacing the live Caddy binary/configuration. Updates preserve both administrator
files. Caddy's normal service user, storage, and renewal automation remain in use.

## Upgrade behavior and verification

Deploy the backend, frontend, and Caddy changes together. Nested legacy hosts
are no longer served: `<slug>--<port>.dev.<host>`, `<id>.apps.<host>`, and
`<slug>.code.<host>` cannot be covered by `*.<host>`. Use the new links generated
by Remote; the built-in IDE remains reachable through `code.<host>/<slug>/` until
its migration. Update bookmarks, preview host allowlists, and public share URLs.
Existing share tokens still refer to the same project/port, but old host-scoped
cookies must be exchanged again at the new host. Browser-local storage does not
move to a different origin. Container files and Code Server settings are unaffected.

Local tests cover routing, access controls, port bounds, candidate preparation,
and configuration validation. Production acceptance also requires a DNS provider
account: verify initial DNS-01 issuance, service restart/renewal, and real app and
preview URLs on the deployment host. No local fixture can prove the provider's
permissions or public DNS propagation. Old cached certificates may remain in
Caddy storage; Remote does not delete them, but the new configuration manages
only the base and wildcard names.
