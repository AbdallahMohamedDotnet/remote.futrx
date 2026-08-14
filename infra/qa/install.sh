#!/usr/bin/env bash
# Exercise the public curl|bash installation flow on a fresh QA server.

set -euo pipefail

QA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck source=common.sh
. "$QA_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage: bash infra/qa/install.sh

Runs the same public curl|bash command documented for a new Remote user. It
refuses to run when /opt/remote.futrx already exists; recreate the server
before using this command to repeat a clean-install test.
EOF
}

case "${1:-}" in
    -h|--help) usage; exit 0 ;;
    '') ;;
    *) usage >&2; exit 2 ;;
esac

qa_prepare_connection
: "${QA_INSTALL_URL:=https://remote.futrx.com/get}"

printf '==> Testing public installation on fresh QA host %s@%s\n' \
    "$QA_SSH_USER" "$QA_SSH_HOST"
printf '    curl -fsSL %s | sudo bash -s -- %s\n' \
    "$QA_INSTALL_URL" "$QA_PUBLIC_HOST"

ssh "${QA_SSH_ARGS[@]}" "$QA_SSH_USER@$QA_SSH_HOST" \
    bash -s -- "$QA_PUBLIC_HOST" "$QA_INSTALL_URL" <<'REMOTE'
set -euo pipefail

public_host="$1"
install_url="$2"
install_dir="/opt/remote.futrx"

if [ -e "$install_dir" ] || systemctl cat remote.futrx.service >/dev/null 2>&1; then
    echo "QA server is not fresh: a remote.futrx installation already exists" >&2
    echo "Use infra/qa/update.sh, or recreate the QA server before testing installation." >&2
    exit 3
fi

curl -fsSL "$install_url" | sudo bash -s -- "$public_host"

cd "$install_dir"
deployed_sha="$(git rev-parse --verify 'HEAD^{commit}')"
systemctl is-active --quiet remote.futrx
. infra/lib/health-check.sh
wait_for_http_health http://127.0.0.1:7682/ 30
printf 'QA_INSTALLED_SHA=%s\n' "$deployed_sha"
REMOTE

qa_verify_public_url

printf '\n✓ QA public installation succeeded\n'
printf '  https://%s/\n' "$QA_PUBLIC_HOST"
