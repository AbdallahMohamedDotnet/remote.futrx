#!/usr/bin/env bash
set -euo pipefail
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TEST_DIR"' EXIT
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/caddy-dns.sh"
err() { echo "$*" >&2; }
log() { :; }
fail() { echo "FAIL: $*" >&2; exit 1; }
mkdir -p "$TEST_DIR/config" "$TEST_DIR/staging"
config="$TEST_DIR/config"
staging="$TEST_DIR/staging"
export CADDY_TEST_MODULE=example CADDY_TEST_LOG="$TEST_DIR/builds"
cat > "$TEST_DIR/caddy" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
    list-modules)
        if [ -f "$0.built" ]; then
            if [ "$#" -gt 1 ]; then
                echo "dns.providers.$CADDY_TEST_MODULE v1.0.0 github.com/caddy-dns/example"
            else
                echo "dns.providers.$CADDY_TEST_MODULE"
            fi
        fi
        exit 0 ;;
    add-package) printf '%s\n' "$2" >> "$CADDY_TEST_LOG"; [ "${CADDY_TEST_FAIL:-0}" = 0 ] || exit 1; touch "$0.built" ;;
esac
MOCK
chmod +x "$TEST_DIR/caddy"
if prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy" 2>/dev/null; then fail "missing provider accepted"; fi
printf 'REMOTE_DNS_PROVIDER=example\nEXAMPLE_TOKEN=secret-must-not-be-logged\n' > "$config/remote-dns.env"
printf 'dns example {env.EXAMPLE_TOKEN}\n' > "$config/remote-dns.caddy"
prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy"
[ "$(cat "$CADDY_TEST_LOG")" = github.com/caddy-dns/example ] || fail "default package not selected"
[ ! -f "$TEST_DIR/caddy.built" ] || fail "live binary was modified"
prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy"
[ "$(wc -l < "$CADDY_TEST_LOG")" = 1 ] || fail "unchanged package rebuilt"
printf 'REMOTE_DNS_PROVIDER=example\nREMOTE_DNS_PACKAGE=example.org/custom/dns@v1.2.3\n' > "$config/remote-dns.env"
prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy"
[ "$(tail -1 "$CADDY_TEST_LOG")" = example.org/custom/dns@v1.2.3 ] || fail "custom package/version ignored"
export CADDY_TEST_FAIL=1
if prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy" 2>/dev/null; then fail "build failure ignored"; fi
export CADDY_TEST_FAIL=0 CADDY_TEST_MODULE=wrong
if prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy" 2>/dev/null; then fail "wrong module accepted"; fi
printf 'REMOTE_DNS_PROVIDER=invalid-name\n' > "$config/remote-dns.env"
if prepare_caddy_dns "$config" "$staging" "$TEST_DIR/caddy" 2>/dev/null; then fail "invalid module accepted"; fi
echo 'Caddy DNS provider tests passed'
