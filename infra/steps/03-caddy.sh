#!/usr/bin/env bash
# Validate a provider-enabled Caddy and the two-name HTTPS configuration before
# replacing live files. Certificate issuance/storage/renewal belong to Caddy.
set -euo pipefail

step_03_caddy() (
    local staging binary="${2:-/usr/local/lib/remote-caddy/caddy}"
    local config_dir="${1:-/etc/caddy}" dropin_dir="${3:-/etc/systemd/system/caddy.service.d}"
    local restart=0
    staging="$(mktemp -d)"
    trap 'rm -rf -- "$staging"' EXIT
    # shellcheck source=../lib/caddy-dns.sh
    . "$INFRA_DIR/lib/caddy-dns.sh"
    prepare_caddy_dns "$config_dir" "$staging" "$binary" || return 1
    render_template "$INFRA_DIR/templates/Caddyfile.tmpl" "$staging/Caddyfile"
    if ! "$staging/caddy" validate --config "$staging/Caddyfile" --adapter caddyfile \
        --envfile "$config_dir/remote-dns.env" >/dev/null 2>&1; then
        err "Invalid Caddy/DNS configuration; live files are unchanged. Check provider options in $config_dir/remote-dns.caddy."
        return 1
    fi
    cat > "$staging/remote-dns.conf" <<EOF
[Service]
EnvironmentFile=$config_dir/remote-dns.env
ExecStart=
ExecStart=$binary run --config $config_dir/Caddyfile --adapter caddyfile
ExecReload=
ExecReload=$binary reload --config $config_dir/Caddyfile --adapter caddyfile --force
EOF
    # Restart for binary/unit/credential changes; otherwise reload so edits to the
    # imported provider snippet take effect even when the template is unchanged.
    if ! cmp -s "$staging/caddy" "$binary" ||
        ! cmp -s "$staging/remote-dns.conf" "$dropin_dir/remote-dns.conf" ||
        [ "$(sha256sum "$config_dir/remote-dns.env" | cut -d ' ' -f1)" != "$(cat "$config_dir/remote-dns.digest" 2>/dev/null || true)" ]; then
        restart=1
    fi
    install -d "$(dirname "$binary")" "$dropin_dir"
    if ! cmp -s "$staging/caddy" "$binary"; then
        install -m 0755 "$staging/caddy" "$binary.new"
        mv -f "$binary.new" "$binary"
    fi
    install -m 0644 "$staging/Caddyfile" "$config_dir/Caddyfile.new"
    mv -f "$config_dir/Caddyfile.new" "$config_dir/Caddyfile"
    install -m 0644 "$staging/remote-dns.conf" "$dropin_dir/remote-dns.conf"
    chmod 0600 "$config_dir/remote-dns.env"
    systemctl daemon-reload
    systemctl enable caddy >/dev/null 2>&1
    if [ "$restart" = 1 ] || ! systemctl is-active --quiet caddy; then
        systemctl restart caddy
    else
        systemctl reload caddy
    fi
    # Store a private fingerprint, never the credential values, for restart detection.
    (umask 077; sha256sum "$config_dir/remote-dns.env" | cut -d ' ' -f1 > "$config_dir/remote-dns.digest")
    ok "Caddy configured for $HOSTNAME and *.$HOSTNAME"
)
