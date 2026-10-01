#!/usr/bin/env bash
set -euo pipefail
INFRA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "$INFRA_DIR/steps/03-caddy.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TEST_DIR"' EXIT
export PATH="$TEST_DIR/bin:$PATH"
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/config" "$TEST_DIR/dropins"
export CADDY_TEST_LOG="$TEST_DIR/systemctl.log"
export HOSTNAME=remote.example.test
cat > "$TEST_DIR/bin/caddy" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
    list-modules)
        if [ "$#" -gt 1 ]; then echo 'dns.providers.example v1.0.0 github.com/caddy-dns/example';
        else echo 'dns.providers.example'; fi ;;
    validate) [ "${FAIL_VALIDATE:-0}" = 0 ] ;;
    *) exit 1 ;;
esac
MOCK
cat > "$TEST_DIR/bin/systemctl" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CADDY_TEST_LOG"
MOCK
chmod +x "$TEST_DIR/bin/"*
log() { :; }; ok() { :; }; err() { echo "$*" >&2; }
render_template() { printf 'rendered-for-%s\n' "$HOSTNAME" > "$2"; }
fail() { echo "FAIL: $*" >&2; exit 1; }
printf 'REMOTE_DNS_PROVIDER=example\nEXAMPLE_TOKEN=first-private-value\n' > "$TEST_DIR/config/remote-dns.env"
printf 'dns example {env.EXAMPLE_TOKEN}\n' > "$TEST_DIR/config/remote-dns.caddy"
printf 'old live configuration\n' > "$TEST_DIR/config/Caddyfile"
export FAIL_VALIDATE=1
if step_03_caddy "$TEST_DIR/config" "$TEST_DIR/managed/caddy" "$TEST_DIR/dropins" 2>/dev/null; then fail 'invalid candidate accepted'; fi
[ "$(cat "$TEST_DIR/config/Caddyfile")" = 'old live configuration' ] || fail 'invalid candidate replaced live config'
[ ! -e "$TEST_DIR/managed/caddy" ] || fail 'invalid candidate installed binary'
[ ! -e "$CADDY_TEST_LOG" ] || fail 'invalid candidate touched service'
export FAIL_VALIDATE=0
step_03_caddy "$TEST_DIR/config" "$TEST_DIR/managed/caddy" "$TEST_DIR/dropins"
grep -qx 'restart caddy' "$CADDY_TEST_LOG" || fail 'new binary did not restart'
[ "$(stat -c %a "$TEST_DIR/config/remote-dns.env")" = 600 ] || fail 'credentials are not private'
! grep -q 'private-value' "$TEST_DIR/config/Caddyfile" "$TEST_DIR/dropins/remote-dns.conf" || fail 'credentials leaked into generated config'
: > "$CADDY_TEST_LOG"
step_03_caddy "$TEST_DIR/config" "$TEST_DIR/managed/caddy" "$TEST_DIR/dropins"
grep -qx 'reload caddy' "$CADDY_TEST_LOG" || fail 'existing binary did not reload'
! grep -q 'restart' "$CADDY_TEST_LOG" || fail 'unchanged credentials caused restart'
printf 'REMOTE_DNS_PROVIDER=example\nEXAMPLE_TOKEN=changed-private-value\n' > "$TEST_DIR/config/remote-dns.env"
: > "$CADDY_TEST_LOG"
step_03_caddy "$TEST_DIR/config" "$TEST_DIR/managed/caddy" "$TEST_DIR/dropins"
grep -qx 'restart caddy' "$CADDY_TEST_LOG" || fail 'credential change did not restart'
echo 'Caddy install tests passed'
