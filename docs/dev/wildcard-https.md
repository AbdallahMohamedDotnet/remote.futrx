# Code Server wildcard HTTPS

Code Server's manifest declares `"web": { "port": 8842, "subdomain": "code" }`.
For project `gamerhead` on `remote.example.com`, it opens at:

```text
https://code--gamerhead.remote.example.com/?folder=%2Fworkspace
```

The manifest label and project slug are dynamic. These single-label application
hosts reuse `*.remote.example.com`; Caddy manages issuance and renewal. The
platform keeps its `remote.example.com` certificate. Opening Code Server in
another project does not request another certificate or call TLS admission.
Remote still checks the session, project membership, and running installation
on every application request. No manual certificate files or export scripts
are needed; Caddy persists its own certificates and renewal state normally.

This change does **not** migrate preview URLs or unnamed applications.
`<slug>--<port>.dev.<host>` and `<instance-id>.apps.<host>` retain their existing
routes, certificates, and admission checks. Consequently, this is not a limit
of two certificates for the entire deployment.

## Administrator-selected DNS provider

Point `*.<public-host>` at the existing Remote ingress. Wildcard issuance needs
DNS-01 credentials and a matching [Caddy DNS module](https://caddyserver.com/docs/modules/).
The administrator chooses the provider; Remote has no default provider.
Before running the installer/updater, create these files on the Remote host:

- `/etc/caddy/remote-dns.env` (root-owned, `0600`): unquoted
  `REMOTE_DNS_PROVIDER=<module-name>`, optional
  `REMOTE_DNS_PACKAGE=<Go-package-path>@<version>`, and the provider's credential
  variables. The default package path is `github.com/caddy-dns/<module-name>`.
- `/etc/caddy/remote-dns.caddy` (root-owned, readable by Caddy): the provider's
  native `dns` directive/options. Reference secrets with `{env.VARIABLE_NAME}`.

For example, **when the administrator chooses Cloudflare**:

```dotenv
# remote-dns.env
REMOTE_DNS_PROVIDER=cloudflare
CLOUDFLARE_API_TOKEN=replace-with-your-zone-dns-token
```

```caddyfile
# remote-dns.caddy
dns cloudflare {env.CLOUDFLARE_API_TOKEN}
```

Other providers may need different options or multiple credentials; use their
module documentation. Credential values use dotenv/systemd-compatible syntax,
with double quotes when needed and no shell expansion.

The installer prepares a copy of the official Caddy binary, adds the selected
module with `caddy add-package`, and validates before replacing live files.
That command uses Caddy's build service and latest Caddy release, with the
requested plugin version when specified. Existing matching modules are reused;
set a newer package version to upgrade. A systemd drop-in uses the managed binary
and private environment file. Credential changes restart Caddy; configuration
changes reload it. Updates preserve both administrator-owned files.

Deploy the core gateway and Caddy changes before the Code Server application.
The existing preview configuration needs no changes. Local routing and installer
tests cover this setup; actual DNS permissions, issuance, and renewal still
require verification on the deployment host. Preserve Caddy's normal storage.
