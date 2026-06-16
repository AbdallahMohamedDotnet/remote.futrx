#!/bin/sh
# Agent Browser launcher — runs inside the project container.
#
# Brings up a real headed Google Chrome on a virtual display and exposes the
# SAME session two ways: a noVNC web view for the user to watch and log in,
# and a loopback CDP port for the agent to drive. The Chrome profile is
# persistent under /workspace so a login the user performs survives container
# restarts. Egress is the container's own (datacenter) network.
#
# Usage: gui-up.sh start|stop|status|restart
set -u

GUI_DIR=/workspace/.browser-gui
PROFILE="$GUI_DIR/profile"
LOG="$GUI_DIR/gui.log"
DISPLAY_NUM=99
VNC_PORT=6080
CDP_PORT=9222
SCREEN=1366x768x24
export DISPLAY=":$DISPLAY_NUM"

CHROME="$(command -v google-chrome 2>/dev/null || echo /usr/bin/google-chrome)"

log() { echo "[gui-up] $*"; }

ready() {
  curl -sf --max-time 2 "http://127.0.0.1:$CDP_PORT/json/version" >/dev/null 2>&1 &&
    curl -sf --max-time 2 "http://127.0.0.1:$VNC_PORT/vnc.html" >/dev/null 2>&1
}

start() {
  mkdir -p "$PROFILE"

  if ! pgrep -f "Xvfb :$DISPLAY_NUM" >/dev/null 2>&1; then
    setsid Xvfb ":$DISPLAY_NUM" -screen 0 "$SCREEN" -ac -nolisten tcp </dev/null >>"$LOG" 2>&1 &
    sleep 1
  fi

  # Window manager first, so Chrome's window gets stable input focus.
  if ! pgrep -x openbox >/dev/null 2>&1; then
    setsid openbox </dev/null >>"$LOG" 2>&1 &
    sleep 1
  fi

  # Headed Chrome, persistent profile, loopback CDP port. --no-sandbox because
  # the unprivileged LXC container is itself the isolation boundary.
  if ! pgrep -f "user-data-dir=$PROFILE" >/dev/null 2>&1; then
    setsid "$CHROME" \
      --user-data-dir="$PROFILE" \
      --no-sandbox --no-first-run --no-default-browser-check \
      --disable-gpu --disable-dev-shm-usage \
      --remote-debugging-port="$CDP_PORT" \
      --window-position=0,0 --window-size=1366,768 \
      "about:blank" </dev/null >>"$LOG" 2>&1 &
    sleep 2
  fi

  # x11vnc on the virtual display, loopback only. -xkb fixes key mapping and
  # -threads keeps input responsive under framebuffer load.
  if ! pgrep -x x11vnc >/dev/null 2>&1; then
    setsid x11vnc -display ":$DISPLAY_NUM" -localhost -forever -shared -nopw \
      -rfbport 5900 -xkb -noxrecord -threads -quiet </dev/null >>"$LOG" 2>&1 &
    sleep 1
  fi

  # noVNC web front, bound to 0.0.0.0 so the host can reach it over the LXD
  # bridge for the dev-URL proxy. It is gated by the platform's Google auth at
  # the Caddy edge; only this port is reachable from outside the container.
  if ! pgrep -f "websockify.*:$VNC_PORT" >/dev/null 2>&1; then
    setsid websockify --web=/usr/share/novnc "0.0.0.0:$VNC_PORT" "127.0.0.1:5900" </dev/null >>"$LOG" 2>&1 &
    sleep 1
  fi

  i=0
  while [ "$i" -lt 30 ]; do
    if ready; then
      log "ready (vnc :$VNC_PORT, cdp :$CDP_PORT)"
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  log "timed out waiting for browser GUI to become ready"
  return 1
}

stop() {
  # pkill patterns below do not match this script's own argv, and pkill never
  # signals itself, so this is safe.
  pkill -f "websockify.*:$VNC_PORT" 2>/dev/null
  pkill -x x11vnc 2>/dev/null
  pkill -f "user-data-dir=$PROFILE" 2>/dev/null
  pkill -x openbox 2>/dev/null
  pkill -f "Xvfb :$DISPLAY_NUM" 2>/dev/null
  log "stopped"
}

case "${1:-start}" in
  start)   start ;;
  stop)    stop ;;
  restart) stop; sleep 1; start ;;
  status)  if ready; then log "ready"; else log "not ready"; fi ;;
  *) echo "usage: $0 start|stop|status|restart" >&2; exit 2 ;;
esac
