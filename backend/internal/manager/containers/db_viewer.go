package containers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const dbViewerTimeout = 180 * time.Second

func (m *Manager) EnsureDBViewer(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	if strings.TrimSpace(containerName) == "" {
		return errors.New("container name required")
	}

	script := fmt.Sprintf(`set -eu
export DEBIAN_FRONTEND=noninteractive
port=%d
root=/var/lib/remote-futrx/db-viewer
pidfile=/run/remote-futrx-adminer.pid
logfile=/var/log/remote-futrx-adminer.log

if ! command -v php >/dev/null 2>&1 || [ ! -f /usr/share/adminer/adminer.php ]; then
  apt-get update -qq
  apt-get install -y -qq --no-install-recommends adminer php-cli php-mysql php-pgsql php-sqlite3
fi

if grep -q 'RemoteFutrxAdminer' /usr/share/adminer/adminer.php 2>/dev/null; then
  apt-get update -qq
  apt-get install -y -qq --reinstall adminer
fi

install -d -m 0755 "$root"
rm -f "$root/index.php"
cat > "$root/index.php" <<'PHP'
<?php
if (!function_exists('adminer_object')) {
    function adminer_object() {
        if (!class_exists('RemoteFutrxAdminer', false)) {
            class RemoteFutrxAdminer extends Adminer {
                function login($login, $password) {
                    return true;
                }
            }
        }
        return new RemoteFutrxAdminer();
    }
}
require "/usr/share/adminer/adminer.php";
PHP

if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
  kill "$(cat "$pidfile")" 2>/dev/null || true
  rm -f "$pidfile"
fi

if timeout 1 bash -c ":</dev/tcp/127.0.0.1/$port" >/dev/null 2>&1; then
  exit 0
fi

nohup php -S "0.0.0.0:$port" -t "$root" >"$logfile" 2>&1 &
echo $! > "$pidfile"

for _ in $(seq 1 40); do
  if timeout 1 bash -c ":</dev/tcp/127.0.0.1/$port" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 0.25
done

cat "$logfile" 2>/dev/null || true
exit 1
`, serviceproject.DBViewerPort)

	dctx, cancel := context.WithTimeout(ctx, dbViewerTimeout)
	defer cancel()
	out, err := m.lxc.Run(dctx, "exec", containerName, "--", "bash", "-lc", script)
	if err != nil {
		return fmt.Errorf("ensure db viewer: %w; output: %s", err, truncateOut(out, 4000))
	}
	return nil
}
