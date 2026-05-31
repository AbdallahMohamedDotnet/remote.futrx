#!/usr/bin/env bash
# Adminer database viewer, fronted by Caddy at db.${HOSTNAME}.
# Auth is delegated to Caddy's forward_auth gate, same as code-server.
#
# Expects from caller:
#   - log / ok helpers
#   - $HOSTNAME
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

log "DB viewer packages (Adminer + PHP-FPM)"
apt-get install -y -qq --no-install-recommends adminer php-fpm php-mysql php-pgsql php-sqlite3

DB_VIEWER_ROOT="/var/www/db.${HOSTNAME}"
install -d -m 0755 "$DB_VIEWER_ROOT"
ln -sfn /usr/share/adminer/adminer.php "$DB_VIEWER_ROOT/index.php"

PHP_FPM_VERSION="$(php -r 'echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;' 2>/dev/null || true)"
PHP_FPM_POOL="/etc/php/${PHP_FPM_VERSION}/fpm/pool.d/www.conf"
if [ -n "$PHP_FPM_VERSION" ] && [ -f "$PHP_FPM_POOL" ]; then
    [ -e "${PHP_FPM_POOL}.orig" ] || cp "$PHP_FPM_POOL" "${PHP_FPM_POOL}.orig"
    sed -i \
        -e 's/^pm = .*/pm = ondemand/' \
        -e 's/^pm.max_children = .*/pm.max_children = 2/' \
        -e 's/^pm.start_servers = /;pm.start_servers = /' \
        -e 's/^pm.min_spare_servers = /;pm.min_spare_servers = /' \
        -e 's/^pm.max_spare_servers = /;pm.max_spare_servers = /' \
        -e 's/^;*pm.process_idle_timeout = .*/pm.process_idle_timeout = 10s/' \
        -e 's/^;*pm.max_requests = .*/pm.max_requests = 200/' \
        "$PHP_FPM_POOL"

    if ! grep -q '^php_admin_value\[memory_limit\]' "$PHP_FPM_POOL"; then
        printf '\nphp_admin_value[memory_limit] = 128M\n' >> "$PHP_FPM_POOL"
    fi

    systemctl enable --now "php${PHP_FPM_VERSION}-fpm" >/dev/null 2>&1 || true
    systemctl restart "php${PHP_FPM_VERSION}-fpm"
fi
ok "Adminer ready at https://db.${HOSTNAME}"
