set -e
# Managed by containers/code_server.go. Installs an on-demand, idle-stopped
# code-server inside a project container. Reached from the host edge at
# <slug>.code.<host> -> <slug>.lxd:8842.
#
# Lifecycle (systemd socket activation -> full scale-to-zero):
#   code-server.socket         listens on 0.0.0.0:8842 (cheap; always armed)
#   code-server-proxy.service  systemd-socket-proxyd --exit-idle-time=10min,
#                              forwards to 127.0.0.1:8081, Requires= the real
#                              service so a first connection pulls it up
#   code-server.service        code-server itself, StopWhenUnneeded=yes so it
#                              stops once the proxy idle-exits
CODE_SERVER_VERSION=4.121.0

if ! command -v code-server >/dev/null 2>&1 \
   || [ "$(code-server --version 2>/dev/null | head -1 | awk '{print $1}')" != "$CODE_SERVER_VERSION" ]; then
    ARCH="$(dpkg --print-architecture)"
    curl -fsSL --retry 3 -o /tmp/code-server.deb \
        "https://github.com/coder/code-server/releases/download/v${CODE_SERVER_VERSION}/code-server_${CODE_SERVER_VERSION}_${ARCH}.deb"
    apt-get install -y -qq /tmp/code-server.deb
    rm -f /tmp/code-server.deb
fi

install -d -m 0700 /root/.config/code-server
cat > /root/.config/code-server/config.yaml <<'YAML'
# code-server listens on loopback only; the socket-activation proxy on :8842
# is the sole reachable port and Caddy's forward_auth gates it, so auth=none
# is safe here (same rationale as the host config).
bind-addr: 127.0.0.1:8081
auth: none
cert: false
app-name: Futrx IDE
YAML
chmod 0600 /root/.config/code-server/config.yaml

cat > /etc/systemd/system/code-server.service <<'UNIT'
[Unit]
Description=code-server (VS Code in the browser) - on-demand
StopWhenUnneeded=yes

[Service]
Type=exec
Environment=VSCODE_RECONNECTION_GRACE_TIME=60000
ExecStart=/usr/bin/code-server
ExecStartPost=/usr/bin/bash -c 'for i in $(seq 1 50); do curl -fsS -o /dev/null http://127.0.0.1:8081/healthz && exit 0; sleep 0.2; done; exit 0'
UNIT

cat > /etc/systemd/system/code-server-proxy.service <<'UNIT'
[Unit]
Description=code-server on-demand proxy (idle-exits and releases the IDE)
Requires=code-server.service
After=code-server.service

[Service]
ExecStart=/usr/lib/systemd/systemd-socket-proxyd --exit-idle-time=10min 127.0.0.1:8081
UNIT

cat > /etc/systemd/system/code-server.socket <<'UNIT'
[Unit]
Description=code-server socket (on-demand activation)

[Socket]
ListenStream=0.0.0.0:8842
Service=code-server-proxy.service

[Install]
WantedBy=sockets.target
UNIT

install -d -m 0755 /root/.local/share/code-server/User
cat > /root/.local/share/code-server/User/settings.json <<'JSON'
{
  "telemetry.telemetryLevel": "off",
  "git.openRepositoryInParentFolders": "always",
  "security.workspace.trust.enabled": false
}
JSON

# Pinned extensions, best-effort: a flaky Open VSX must never fail the build.
for ext in \
    anan.jetbrains-darcula-theme \
    pkief.material-icon-theme \
    golang.go \
    dbcode.dbcode \
    ; do
    code-server --install-extension "$ext" >/dev/null 2>&1 || true
done

systemctl daemon-reload 2>/dev/null || true
systemctl enable code-server.socket >/dev/null 2>&1 || true
