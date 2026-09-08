#!/usr/bin/env bash
# Verifies the /usr/local/bin/remote launcher and its canonical config file:
# rendering, argument/stdio/exit-code forwarding, process replacement via
# exec, default and overridden install directories, and failure on missing or
# incomplete configuration. Also checks that install.sh and the systemd unit
# template wire the same two artifacts instead of duplicating configuration.

set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
INFRA_DIR="$(cd "$TESTS_DIR/.." >/dev/null 2>&1 && pwd)"
CLI_TEMPLATE="$INFRA_DIR/templates/remote-cli.sh.tmpl"
ENV_TEMPLATE="$INFRA_DIR/templates/remote.futrx.env.tmpl"
SERVICE_TEMPLATE="$INFRA_DIR/templates/remote.futrx.service.tmpl"
INSTALLER="$INFRA_DIR/install.sh"
BACKEND_SVC_STEP="$INFRA_DIR/steps/04-backend-svc.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TEST_DIR="$(mktemp -d)"
trap 'command rm -rf -- "$TEST_DIR"' EXIT

# ───────────────── syntax + static wiring ─────────────────
bash -n "$CLI_TEMPLATE" || fail "launcher template has a syntax error"
bash -n "$BACKEND_SVC_STEP" || fail "backend-svc step has a syntax error"

grep -Fq 'ExecStart=/usr/local/bin/remote' "$SERVICE_TEMPLATE" || \
    fail "systemd unit does not launch through the shared CLI entry point"
for stale_env in 'Environment=DATA_DIR=' 'Environment=INSTALL_DIR=' 'Environment=BASE_URL='; do
    if grep -Fq "$stale_env" "$SERVICE_TEMPLATE"; then
        fail "systemd unit still duplicates $stale_env instead of reading the shared config file"
    fi
done
grep -Fq 'REMOTE_ENV_FILE="${FUTRX_REMOTE_ENV_FILE:-/etc/default/remote.futrx}"' "$INSTALLER" || \
    fail "installer does not define the canonical config path"
grep -Fq 'REMOTE_CLI_PATH="${FUTRX_REMOTE_CLI_PATH:-/usr/local/bin/remote}"' "$INSTALLER" || \
    fail "installer does not define the canonical CLI path"
grep -Fq 'install -o root -g root -m 0755' "$BACKEND_SVC_STEP" || \
    fail "backend-svc step does not install the launcher as root:root 0755"
grep -Fq 'chmod 0644 "$REMOTE_ENV_FILE"' "$BACKEND_SVC_STEP" || \
    fail "backend-svc step does not set 0644 on the canonical config file"

# ───────────────── render the canonical config the way install.sh does ─────
# Mirrors install.sh's render_template whitelist without sourcing the whole
# installer (which parses argv and requires root).
render_env() {
    local hostname="$1" install_dir="$2" dest="$3"
    HOSTNAME="$hostname" INSTALL_DIR="$install_dir" \
        envsubst '$HOSTNAME $INSTALL_DIR' < "$ENV_TEMPLATE" > "$dest"
}

DEFAULT_ENV="$TEST_DIR/default.env"
render_env "remote.example.com" "/opt/remote.futrx" "$DEFAULT_ENV"
grep -Fxq 'BASE_URL=https://remote.example.com' "$DEFAULT_ENV" || \
    fail "default config did not render BASE_URL"
grep -Fxq 'DATA_DIR=/opt/remote.futrx/data' "$DEFAULT_ENV" || \
    fail "default config did not render DATA_DIR"
grep -Fxq 'INSTALL_DIR=/opt/remote.futrx' "$DEFAULT_ENV" || \
    fail "default config did not render INSTALL_DIR"

OVERRIDE_ENV="$TEST_DIR/override.env"
render_env "qa.example.com" "/srv/remote-qa" "$OVERRIDE_ENV"
grep -Fxq 'BASE_URL=https://qa.example.com' "$OVERRIDE_ENV" || \
    fail "overridden config did not render BASE_URL"
grep -Fxq 'DATA_DIR=/srv/remote-qa/data' "$OVERRIDE_ENV" || \
    fail "overridden config did not render DATA_DIR"
grep -Fxq 'INSTALL_DIR=/srv/remote-qa' "$OVERRIDE_ENV" || \
    fail "overridden config did not render INSTALL_DIR"

# ───────────────── install the launcher into a scratch bin ─────────────────
CLI_BIN="$TEST_DIR/remote"
cp "$CLI_TEMPLATE" "$CLI_BIN"
chmod 0755 "$CLI_BIN"

install_dir="$TEST_DIR/opt/remote.futrx"
mkdir -p "$install_dir/backend" "$install_dir/data"
BACKEND_FAKE="$install_dir/backend/remote"

# A fake backend that proves: cwd, forwarded argv (with a space and a shell
# metacharacter), forwarded env vars, stdout+stderr forwarding, and exit code.
cat > "$BACKEND_FAKE" <<'FAKE'
#!/usr/bin/env bash
echo "cwd=$PWD"
echo "args=$#"
i=0
for a in "$@"; do
    i=$((i + 1))
    echo "arg${i}=${a}"
done
echo "BASE_URL=${BASE_URL:-}"
echo "DATA_DIR=${DATA_DIR:-}"
echo "INSTALL_DIR=${INSTALL_DIR:-}"
echo "stderr line" >&2
[ "${FAKE_EXIT:-0}" -eq 0 ] || exit "${FAKE_EXIT}"
FAKE
chmod 0755 "$BACKEND_FAKE"

render_env "remote.example.com" "$install_dir" "$TEST_DIR/scratch.env"

if ! FUTRX_REMOTE_ENV_FILE="$TEST_DIR/scratch.env" \
    "$CLI_BIN" setup-token "has space" 'weird;`$(rm)' \
    >"$TEST_DIR/out.log" 2>"$TEST_DIR/err.log"; then
    fail "launcher exited non-zero on a backend that exits 0"
fi
grep -Fxq "cwd=$install_dir" "$TEST_DIR/out.log" || \
    fail "launcher did not cd into INSTALL_DIR before exec"
grep -Fxq 'arg1=setup-token' "$TEST_DIR/out.log" || fail "first argument was not forwarded"
grep -Fxq 'arg2=has space' "$TEST_DIR/out.log" || fail "argument with whitespace was not forwarded intact"
grep -Fxq 'arg3=weird;`$(rm)' "$TEST_DIR/out.log" || \
    fail "argument with shell metacharacters was not forwarded intact (eval-free)"
grep -Fxq "BASE_URL=https://remote.example.com" "$TEST_DIR/out.log" || \
    fail "BASE_URL was not exported to the backend"
grep -Fxq "DATA_DIR=$install_dir/data" "$TEST_DIR/out.log" || \
    fail "DATA_DIR was not exported to the backend"
grep -Fxq "INSTALL_DIR=$install_dir" "$TEST_DIR/out.log" || \
    fail "INSTALL_DIR was not exported to the backend"
grep -Fxq 'stderr line' "$TEST_DIR/err.log" || fail "stderr was not forwarded"

# Exit code forwarding.
set +e
FUTRX_REMOTE_ENV_FILE="$TEST_DIR/scratch.env" FAKE_EXIT=7 "$CLI_BIN" >/dev/null 2>&1
status=$?
set -e
[ "$status" -eq 7 ] || fail "launcher did not forward the backend's exit code (got $status)"

# Process replacement: the launcher's own PID becomes the backend's PID.
LAUNCHER_PID="$TEST_DIR/pid.log"
cat > "$install_dir/backend/remote" <<PIDCHECK
#!/usr/bin/env bash
echo "\$\$" > "$LAUNCHER_PID"
PIDCHECK
chmod 0755 "$install_dir/backend/remote"
FUTRX_REMOTE_ENV_FILE="$TEST_DIR/scratch.env" "$CLI_BIN" &
child_pid=$!
wait "$child_pid"
reported_pid="$(cat "$LAUNCHER_PID")"
[ "$reported_pid" = "$child_pid" ] || \
    fail "exec did not replace the launcher process (backend pid $reported_pid != launched pid $child_pid)"

# ───────────────── failure modes ─────────────────
if FUTRX_REMOTE_ENV_FILE="$TEST_DIR/does-not-exist.env" "$CLI_BIN" >/dev/null 2>"$TEST_DIR/missing.log"; then
    fail "launcher did not fail with a missing config file"
fi
grep -q "missing configuration" "$TEST_DIR/missing.log" || \
    fail "missing-config error was not reported clearly"

INCOMPLETE_ENV="$TEST_DIR/incomplete.env"
printf 'BASE_URL=https://remote.example.com\n' > "$INCOMPLETE_ENV"
if FUTRX_REMOTE_ENV_FILE="$INCOMPLETE_ENV" "$CLI_BIN" >/dev/null 2>"$TEST_DIR/incomplete.log"; then
    fail "launcher did not fail with an incomplete config file"
fi
grep -q "missing BASE_URL, DATA_DIR, or INSTALL_DIR" "$TEST_DIR/incomplete.log" || \
    fail "incomplete-config error was not reported clearly"

MISSING_BACKEND_ENV="$TEST_DIR/missing-backend.env"
render_env "remote.example.com" "$TEST_DIR/no-such-install" "$MISSING_BACKEND_ENV"
if FUTRX_REMOTE_ENV_FILE="$MISSING_BACKEND_ENV" "$CLI_BIN" >/dev/null 2>"$TEST_DIR/no-backend.log"; then
    fail "launcher did not fail when the backend binary is missing"
fi
grep -q "backend binary not found" "$TEST_DIR/no-backend.log" || \
    fail "missing-backend error was not reported clearly"

# ───────────────── repair: overwrite a stale/incorrect entry point ─────────
STALE_CLI="$TEST_DIR/usr-local-bin-remote"
printf '#!/usr/bin/env bash\necho stale\n' > "$STALE_CLI"
chmod 0755 "$STALE_CLI"
cp "$CLI_TEMPLATE" "$STALE_CLI.tmp"
bash -n "$STALE_CLI.tmp" || fail "re-rendered launcher failed its own syntax check"
install -m 0755 "$STALE_CLI.tmp" "$STALE_CLI"
grep -Fq 'exec "$BACKEND_BIN"' "$STALE_CLI" || fail "repair did not replace a stale launcher"

echo "remote CLI entrypoint tests passed"
