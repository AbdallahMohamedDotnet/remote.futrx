#!/usr/bin/env bash
# code-server: VS Code in the browser, fronted by Caddy at code.${HOSTNAME}.
# Auth is delegated entirely to Caddy's forward_auth gate — code-server runs
# with `auth: none` and binds only to loopback.
#
# Installation: official .deb from coder/code-server releases. We pin a
# version so deploys are deterministic; bump CODE_SERVER_VERSION below to
# upgrade and re-run.
#
# Besides the binary this step also manages the user-level experience:
# templates/code-server-settings.json is merged into the live settings.json
# (managed keys win, runtime keys like dbcode.* are preserved) and the
# CODE_SERVER_EXTENSIONS list below is guaranteed installed.
#
# Expects from caller:
#   - log / ok / warn helpers
#   - $INFRA_DIR, $CODE_SERVER_PORT
set -euo pipefail

CODE_SERVER_VERSION="${CODE_SERVER_VERSION:-4.121.0}"

# Extensions guaranteed present (Open VSX ids). Additive only: extensions
# installed by hand stay; removing an id here does not uninstall it.
CODE_SERVER_EXTENSIONS=(
    anan.jetbrains-darcula-theme
    anwar.papyrus-pdf
    chuckjonas.duckdb
    dbcode.dbcode
    golang.go
    onlyutkarsh.mermaid-diagram-lens
    pkief.material-icon-theme
    repreng.csv
)

# ───────────────── install (skip if version matches) ─────────────────
CURRENT_VERSION=""
if command -v code-server >/dev/null; then
    CURRENT_VERSION="$(code-server --version 2>/dev/null | head -1 | awk '{print $1}' || true)"
fi

if [ "$CURRENT_VERSION" != "$CODE_SERVER_VERSION" ]; then
    log "Installing code-server $CODE_SERVER_VERSION (current: ${CURRENT_VERSION:-none})"
    # ARM (cax11) is arm64; AMD64 boxes are amd64. dpkg --print-architecture
    # tells us what Debian expects, which matches code-server's release naming.
    ARCH="$(dpkg --print-architecture)"
    URL="https://github.com/coder/code-server/releases/download/v${CODE_SERVER_VERSION}/code-server_${CODE_SERVER_VERSION}_${ARCH}.deb"
    TMP_DEB="$(mktemp --suffix=.deb)"
    trap 'rm -f "$TMP_DEB"' RETURN
    curl -fsSL --retry 3 -o "$TMP_DEB" "$URL"
    apt-get install -y -qq "$TMP_DEB"
fi
ok "code-server $(code-server --version | head -1 | awk '{print $1}')"

# ───────────────── config render ─────────────────
# Always re-render on each run so template changes propagate immediately.
mkdir -p /root/.config/code-server
render_template "${INFRA_DIR}/templates/code-server-config.yaml.tmpl" \
                /root/.config/code-server/config.yaml
chmod 0600 /root/.config/code-server/config.yaml

# ───────────────── user settings ─────────────────
# Merge the managed baseline into the live settings.json. Baseline keys
# always win — to change a managed setting, edit the template and re-run,
# not the code-server UI. Keys absent from the baseline (dbcode
# connections/tunnels and other extension-written runtime state) are
# preserved untouched. The merged file on disk is plain JSON; comments
# live only in the repo copy.
USER_DIR=/root/.local/share/code-server/User
mkdir -p "$USER_DIR"
python3 - "${INFRA_DIR}/templates/code-server-settings.json" "${USER_DIR}/settings.json" <<'PY'
import json, re, sys
from pathlib import Path

def parse_jsonc(text):
    """json.loads with // and /* */ comments and trailing commas allowed."""
    out, i, n, in_str = [], 0, len(text), False
    while i < n:
        c = text[i]
        if in_str:
            out.append(c)
            if c == "\\" and i + 1 < n:
                out.append(text[i + 1]); i += 2; continue
            if c == '"':
                in_str = False
            i += 1
        elif c == '"':
            in_str = True; out.append(c); i += 1
        elif text[i:i + 2] == "//":
            while i < n and text[i] != "\n":
                i += 1
        elif text[i:i + 2] == "/*":
            end = text.find("*/", i + 2)
            i = n if end == -1 else end + 2
        else:
            out.append(c); i += 1
    return json.loads(re.sub(r",(\s*[}\]])", r"\1", "".join(out)))

baseline = parse_jsonc(Path(sys.argv[1]).read_text())
dest = Path(sys.argv[2])
existing = parse_jsonc(dest.read_text()) if dest.exists() else {}
merged = {**existing, **baseline}
dest.write_text(json.dumps(merged, indent=2) + "\n")
PY
ok "settings.json: managed baseline merged"

# ───────────────── extensions ─────────────────
INSTALLED_EXTS="$(code-server --list-extensions 2>/dev/null || true)"
for ext in "${CODE_SERVER_EXTENSIONS[@]}"; do
    if ! grep -qixF "$ext" <<<"$INSTALLED_EXTS"; then
        log "Installing extension: $ext"
        code-server --install-extension "$ext" >/dev/null
    fi
done
ok "${#CODE_SERVER_EXTENSIONS[@]} pinned extensions present"

# ───────────────── systemd ─────────────────
# The Debian package ships a template unit code-server@.service that runs as
# the user passed as the instance name. We use code-server@root (root being
# the install user) which is the same convention as the rest of this box.
if ! systemctl is-enabled --quiet code-server@root.service 2>/dev/null; then
    log "Enabling code-server@root.service"
    systemctl enable code-server@root.service >/dev/null
fi

# Restart picks up config changes. Cheap (~1s) and graceful.
log "Restarting code-server@root.service"
systemctl restart code-server@root.service
sleep 1
if ! systemctl is-active --quiet code-server@root.service; then
    err "code-server failed to start. Recent logs:"
    journalctl -u code-server@root.service -n 20 --no-pager >&2
    exit 1
fi
ok "code-server@root.service is active on 127.0.0.1:${CODE_SERVER_PORT}"
