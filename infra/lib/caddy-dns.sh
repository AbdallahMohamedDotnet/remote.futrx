#!/usr/bin/env bash
# Prepare a provider-enabled candidate without replacing the running binary.
# The administrator owns remote-dns.env and remote-dns.caddy; updates preserve both.
prepare_caddy_dns() {
    local config_dir="$1" staging_dir="$2" installed_binary="$3"
    local env_file="$config_dir/remote-dns.env" provider package seed metadata version package_path
    if [ ! -r "$env_file" ] || [ ! -s "$config_dir/remote-dns.caddy" ]; then
        err "Wildcard HTTPS requires $env_file and $config_dir/remote-dns.caddy. See docs/dev/wildcard-https.md."
        return 1
    fi
    # Read only the two non-secret settings. Never source this file or print it.
    provider="$(sed -n 's/^REMOTE_DNS_PROVIDER=\([a-z0-9_]*\)$/\1/p' "$env_file")"
    package="$(sed -n 's/^REMOTE_DNS_PACKAGE=\([^[:space:]]*\)$/\1/p' "$env_file")"
    if [[ ! "$provider" =~ ^[a-z][a-z0-9_]*$ ]]; then
        err "Set REMOTE_DNS_PROVIDER to the Caddy DNS module name in $env_file."
        return 1
    fi
    package="${package:-github.com/caddy-dns/$provider}"
    if [[ ! "$package" =~ ^[a-zA-Z0-9._/-]+(@[a-zA-Z0-9._+-]+)?$ ]]; then
        err "REMOTE_DNS_PACKAGE must be a Go package path, optionally followed by @version."
        return 1
    fi
    seed="$installed_binary"
    [ -x "$seed" ] || seed="$(command -v caddy)"
    cp "$seed" "$staging_dir/caddy" || return 1
    # Inspect the embedded package metadata: add-package refuses a package
    # already present at the requested version. This also supports custom seeds.
    metadata="$("$staging_dir/caddy" list-modules --skip-standard --packages --versions |
        awk -v module="dns.providers.$provider" '$1 == module { print $2 " " $3 }')" || return 1
    version="${metadata%% *}"
    package_path="${metadata#* }"
    if [ "$package_path" != "${package%@*}" ] ||
        { [[ "$package" == *@* ]] && [ "$version" != "${package##*@}" ]; }; then
        log "Installing the selected Caddy DNS module: $provider"
        if ! "$staging_dir/caddy" add-package "$package" >/dev/null 2>&1; then
            err "Could not build Caddy with $package; the live binary is unchanged."
            return 1
        fi
    fi
    if ! "$staging_dir/caddy" list-modules | grep -x "dns.providers.$provider" >/dev/null; then
        err "The selected package does not provide dns.providers.$provider."
        return 1
    fi
}
