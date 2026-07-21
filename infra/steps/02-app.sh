#!/usr/bin/env bash
# Repo clone/update + frontend & backend build + Google OAuth config seed.
#
# This step is normally a no-op on CI deploy (the repo's already cloned and
# CI does the pull itself) but stays here so the first-time installer works
# end-to-end from curl|bash.
#
# Expects from caller:
#   - log / ok / err helpers
#   - $INSTALL_DIR, $REPO_URL, $GITHUB_TOKEN, $HOSTNAME
#   - $GOOGLE_CLIENT_ID, $GOOGLE_CLIENT_SECRET (optional)
#
# Sets:
#   - $AUTH_NOTE — string for the install summary
set -euo pipefail

# ───────────────── clone / update ─────────────────
if [ -d "$INSTALL_DIR" ] && [ ! -d "$INSTALL_DIR/.git" ]; then
    err "$INSTALL_DIR exists but is not a git checkout. Remove it and re-run."
    exit 1
fi

CLONE_URL="$REPO_URL"
if [ -n "${GITHUB_TOKEN:-}" ]; then
    CLONE_URL="https://x-access-token:${GITHUB_TOKEN}@github.com/futrx-com/remote.futrx.com.git"
fi

if [ -d "$INSTALL_DIR/.git" ]; then
    log "Updating repo at $INSTALL_DIR"
    git -C "$INSTALL_DIR" fetch --quiet --tags origin
    git -C "$INSTALL_DIR" reset --hard origin/main
else
    log "Cloning repo to $INSTALL_DIR"
    if ! git clone --depth=1 "$CLONE_URL" "$INSTALL_DIR" 2>&1; then
        err "git clone failed."
        if [ -z "${GITHUB_TOKEN:-}" ]; then
            cat <<EOF >&2

  This repo is private. Provide a GitHub Personal Access Token:
    1. https://github.com/settings/personal-access-tokens → Generate (fine-grained)
    2. Select repo futrx-com/remote.futrx.com → Contents: Read
    3. Re-run: sudo $0 $HOSTNAME --github-token=ghp_xxx
EOF
        fi
        exit 1
    fi
    chmod 0600 "$INSTALL_DIR/.git/config"
fi
cd "$INSTALL_DIR"

# ───────────────── build ─────────────────
log "Building frontend (frontend/ → backend/public/)"
(
    cd frontend
    npm install --silent --no-audit --no-fund 2>&1 | tail -3
    npm run build 2>&1 | tail -5
)

log "Building backend (Go → backend/remote)"
(
    cd backend
    go build -trimpath -ldflags="-s -w" -o remote ./cmd/remote
)
ok "$(ls -lh backend/remote | awk '{print $5}') binary"

# ───────────────── data dir + OAuth seed ─────────────────
mkdir -p "$INSTALL_DIR/data"
chmod 0700 "$INSTALL_DIR/data"

if [ -n "${GOOGLE_CLIENT_ID:-}" ] && [ -n "${GOOGLE_CLIENT_SECRET:-}" ]; then
    log "Writing Google OAuth config (data/oauth.json)"
    SECRET_ESC=$(printf '%s' "$GOOGLE_CLIENT_SECRET" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))' 2>/dev/null \
                 || printf '%s' "\"$GOOGLE_CLIENT_SECRET\"")
    cat > "$INSTALL_DIR/data/oauth.json" <<EOF
{
  "googleClientId": "$GOOGLE_CLIENT_ID",
  "googleClientSecret": $SECRET_ESC
}
EOF
    chmod 0600 "$INSTALL_DIR/data/oauth.json"
    ok "Google sign-in enabled for invited users"
    AUTH_NOTE="Local admin authentication enabled. Google sign-in is ready for invited users."
elif [ -s "$INSTALL_DIR/data/oauth.json" ]; then
    ok "Google sign-in enabled for invited users (using the existing configuration)"
    AUTH_NOTE="Local admin authentication enabled. Existing Google user sign-in settings were preserved."
else
    AUTH_NOTE="Local admin authentication enabled. Create the admin password on first visit."
fi
export AUTH_NOTE
