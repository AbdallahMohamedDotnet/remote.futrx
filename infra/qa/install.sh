#!/usr/bin/env bash
# QA wrapper for exercising installation on a fresh, disposable server.
#
# It reads SSH/public-host settings from .qa.env (see .qa.env.example), refuses
# a server with an existing Remote installation, performs the installation over
# SSH, and verifies both local and public health. It does not recreate or clean
# the QA server for you.

set -euo pipefail

QA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck source=common.sh
. "$QA_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage: bash infra/qa/install.sh [branch|tag|commit]

With no ref, runs the same public curl|bash command documented for a new Remote
user. With a ref, resolves the pushed ref to an immutable commit and
curl-installs that exact candidate commit. It refuses to run when
/opt/remote.futrx already exists; recreate the server before using this command
to repeat a clean-install test.

Candidate mode requires a clean local checkout whose HEAD matches the requested
ref on origin. Connection settings come from .qa.env (see .qa.env.example).
EOF
}

if [ "$#" -gt 1 ]; then
    usage >&2
    exit 2
fi
case "${1:-}" in
    -h|--help) usage; exit 0 ;;
esac

: "${QA_INSTALL_URL:=https://remote.futrx.com/get}"
CANDIDATE_SHA=""
if [ "$#" -eq 1 ]; then
    qa_prepare "$1"
    CANDIDATE_SHA="$QA_CANDIDATE_SHA"
    QA_INSTALL_URL="https://raw.githubusercontent.com/futrx-com/remote.futrx/${CANDIDATE_SHA}/infra/install.sh"
else
    qa_prepare_connection
fi

if [ -n "$CANDIDATE_SHA" ]; then
    printf '==> Testing candidate installation of %s on %s@%s\n' \
        "$CANDIDATE_SHA" "$QA_SSH_USER" "$QA_SSH_HOST"
    printf '    curl -fsSL %s | sudo bash -s -- %s --ref=%s\n' \
        "$QA_INSTALL_URL" "$QA_PUBLIC_HOST" "$CANDIDATE_SHA"
else
    printf '==> Testing public installation on fresh QA host %s@%s\n' \
        "$QA_SSH_USER" "$QA_SSH_HOST"
    printf '    curl -fsSL %s | sudo bash -s -- %s\n' \
        "$QA_INSTALL_URL" "$QA_PUBLIC_HOST"
fi

ssh "${QA_SSH_ARGS[@]}" "$QA_SSH_USER@$QA_SSH_HOST" \
    bash -s -- "$QA_PUBLIC_HOST" "$QA_INSTALL_URL" "$CANDIDATE_SHA" <<'REMOTE'
set -euo pipefail

public_host="$1"
install_url="$2"
candidate_sha="$3"
install_dir="/opt/remote.futrx"

if [ -e "$install_dir" ] || systemctl cat remote.futrx.service >/dev/null 2>&1; then
    echo "QA server is not fresh: a remote.futrx installation already exists" >&2
    echo "Use infra/qa/update.sh, or recreate the QA server before testing installation." >&2
    exit 3
fi

if [ -n "$candidate_sha" ]; then
    curl -fsSL "$install_url" | sudo bash -s -- "$public_host" "--ref=$candidate_sha"
else
    curl -fsSL "$install_url" | sudo bash -s -- "$public_host"
fi

cd "$install_dir"
deployed_sha="$(git rev-parse --verify 'HEAD^{commit}')"
if [ -n "$candidate_sha" ] && [ "$deployed_sha" != "$candidate_sha" ]; then
    echo "installed $deployed_sha, expected candidate $candidate_sha" >&2
    exit 1
fi
systemctl is-active --quiet remote.futrx
. infra/lib/health-check.sh
wait_for_http_health http://127.0.0.1:7682/ 30

command -v remote >/dev/null 2>&1 || { echo "remote CLI entry point is not on PATH" >&2; exit 1; }

cli_path="$(command -v remote)"
[ "$cli_path" = "/usr/local/bin/remote" ] || {
    echo "remote resolved to $cli_path, expected /usr/local/bin/remote" >&2
    exit 1
}
cli_owner="$(stat -c '%U:%G' /usr/local/bin/remote)"
cli_mode="$(stat -c '%a' /usr/local/bin/remote)"
[ "$cli_owner" = "root:root" ] && [ "$cli_mode" = "755" ] || {
    echo "/usr/local/bin/remote has wrong owner/mode: $cli_owner $cli_mode" >&2
    exit 1
}
env_owner="$(stat -c '%U:%G' /etc/default/remote.futrx)"
env_mode="$(stat -c '%a' /etc/default/remote.futrx)"
[ "$env_owner" = "root:root" ] && [ "$env_mode" = "644" ] || {
    echo "/etc/default/remote.futrx has wrong owner/mode: $env_owner $env_mode" >&2
    exit 1
}

setup_out="$(sudo remote setup-token)"
echo "$setup_out" | grep -qF "https://$public_host/?token=" || {
    echo "setup-token did not print the expected URL:" >&2
    echo "$setup_out" >&2
    exit 1
}
[ -f "$install_dir/data/setup-token.json" ] || {
    echo "setup-token.json was not written under $install_dir/data" >&2
    exit 1
}
first_hash="$(sha256sum "$install_dir/data/setup-token.json")"
sudo remote setup-token >/dev/null
second_hash="$(sha256sum "$install_dir/data/setup-token.json")"
[ "$first_hash" != "$second_hash" ] || {
    echo "reissuing the setup token did not change the stored record" >&2
    exit 1
}

systemctl is-active --quiet remote.futrx
wait_for_http_health http://127.0.0.1:7682/ 30
printf 'QA_INSTALLED_SHA=%s\n' "$deployed_sha"
REMOTE

qa_verify_public_url

if [ -n "$CANDIDATE_SHA" ]; then
    printf '\n✓ QA candidate installation succeeded at %s\n' "$CANDIDATE_SHA"
else
    printf '\n✓ QA public installation succeeded\n'
fi
printf '  https://%s/\n' "$QA_PUBLIC_HOST"
