#!/usr/bin/env bash
# Shared local safeguards for the explicit QA install and update commands.

set -euo pipefail

QA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
QA_REPO_ROOT="$(cd "$QA_DIR/../.." >/dev/null 2>&1 && pwd)"
QA_ENV_FILE="${QA_ENV_FILE:-$QA_REPO_ROOT/.qa.env}"

qa_fail() {
    printf 'qa: %s\n' "$*" >&2
    exit 1
}

qa_prepare_connection() {
    local command_name resolved_qa_ip

    if [ ! -r "$QA_ENV_FILE" ]; then
        qa_fail "missing $QA_ENV_FILE; copy .qa.env.example to .qa.env and configure it"
    fi

    # shellcheck source=/dev/null
    . "$QA_ENV_FILE"

    : "${QA_SSH_HOST:?set QA_SSH_HOST in $QA_ENV_FILE}"
    : "${QA_SSH_USER:=root}"
    : "${QA_SSH_KEY:?set QA_SSH_KEY in $QA_ENV_FILE}"
    : "${QA_KNOWN_HOSTS_FILE:?set QA_KNOWN_HOSTS_FILE in $QA_ENV_FILE}"
    : "${QA_PUBLIC_HOST:?set QA_PUBLIC_HOST in $QA_ENV_FILE}"

    [ -r "$QA_SSH_KEY" ] || qa_fail "SSH private key is not readable: $QA_SSH_KEY"
    mkdir -p "$(dirname "$QA_KNOWN_HOSTS_FILE")"
    touch "$QA_KNOWN_HOSTS_FILE"
    chmod 600 "$QA_KNOWN_HOSTS_FILE"

    for command_name in ssh curl getent; do
        command -v "$command_name" >/dev/null 2>&1 || qa_fail "required command is missing: $command_name"
    done

    resolved_qa_ip="$(getent ahostsv4 "$QA_PUBLIC_HOST" 2>/dev/null | awk 'NR == 1 { print $1 }')"
    if [ -z "$resolved_qa_ip" ]; then
        qa_fail "$QA_PUBLIC_HOST does not resolve to an IPv4 address"
    fi
    if [ "$QA_SSH_HOST" != "$QA_PUBLIC_HOST" ] && [ "$resolved_qa_ip" != "$QA_SSH_HOST" ]; then
        qa_fail "$QA_PUBLIC_HOST resolves to $resolved_qa_ip, not QA_SSH_HOST $QA_SSH_HOST"
    fi

    QA_SSH_ARGS=(
        -i "$QA_SSH_KEY"
        -o BatchMode=yes
        -o ConnectTimeout=10
        -o StrictHostKeyChecking=yes
        -o "UserKnownHostsFile=$QA_KNOWN_HOSTS_FILE"
    )
}

qa_prepare() {
    local requested_ref="$1" command_name test_script

    case "$requested_ref" in
        '') qa_fail "a branch, tag, or commit is required" ;;
        *[!A-Za-z0-9._/@+-]*) qa_fail "ref contains unsupported characters: $requested_ref" ;;
    esac

    qa_prepare_connection

    for command_name in git npm go; do
        command -v "$command_name" >/dev/null 2>&1 || qa_fail "required command is missing: $command_name"
    done

    cd "$QA_REPO_ROOT"

    if ! git diff --quiet || ! git diff --cached --quiet; then
        qa_fail "tracked working-tree changes exist; commit or stash them before deploying"
    fi

    printf '==> Fetching %s from origin\n' "$requested_ref"
    git fetch --quiet origin "$requested_ref"
    QA_CANDIDATE_SHA="$(git rev-parse --verify 'FETCH_HEAD^{commit}')"
    QA_LOCAL_SHA="$(git rev-parse --verify 'HEAD^{commit}')"
    QA_REQUESTED_REF="$requested_ref"

    if [ "$QA_LOCAL_SHA" != "$QA_CANDIDATE_SHA" ]; then
        qa_fail "local HEAD $QA_LOCAL_SHA is not the pushed candidate $QA_CANDIDATE_SHA; check out the requested ref first"
    fi

    printf '==> Candidate: %s (%s)\n' "$QA_REQUESTED_REF" "$QA_CANDIDATE_SHA"

    printf '==> Running infrastructure tests\n'
    for test_script in "$QA_REPO_ROOT"/infra/tests/*.sh; do
        bash "$test_script"
    done

    printf '==> Running frontend tests and build\n'
    npm --prefix "$QA_REPO_ROOT/frontend" ci --no-audit --no-fund
    npm --prefix "$QA_REPO_ROOT/frontend" test
    npm --prefix "$QA_REPO_ROOT/frontend" run build

    printf '==> Running Go tests and vet\n'
    (
        cd "$QA_REPO_ROOT/backend"
        go test ./...
        go vet ./...
    )
}

qa_verify_public_url() {
    printf '==> Verifying https://%s/\n' "$QA_PUBLIC_HOST"
    curl -fsS --max-time 20 "https://$QA_PUBLIC_HOST/" >/dev/null
}
