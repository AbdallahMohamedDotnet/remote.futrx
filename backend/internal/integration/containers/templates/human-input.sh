#!/bin/sh
# OS-level input fallback for the Agent Browser X display.
#
# Coordinates are viewport pixels: the Chrome window is positioned at 0,0 and
# sized to the Xvfb screen (1366x768).
set -eu

export DISPLAY="${DISPLAY:-:99}"
DELAY="${HUMAN_INPUT_DELAY_MS:-35}"

usage() {
  cat >&2 <<'EOF'
usage:
  human-input.sh move <x> <y>
  human-input.sh click <x> <y> [button]
  human-input.sh type <text>
  human-input.sh key <keysym> [keysym...]
  human-input.sh scroll <ticks>
EOF
  exit 2
}

need_xdotool() {
  command -v xdotool >/dev/null 2>&1 || {
    echo "xdotool is not installed" >&2
    exit 1
  }
}

move_mouse() {
  [ "$#" -eq 2 ] || usage
  target_x="$1"
  target_y="$2"
  eval "$(xdotool getmouselocation --shell 2>/dev/null || echo 'X=0 Y=0')"
  steps=12
  i=1
  while [ "$i" -le "$steps" ]; do
    next_x=$(( X + (target_x - X) * i / steps ))
    next_y=$(( Y + (target_y - Y) * i / steps ))
    xdotool mousemove "$next_x" "$next_y"
    sleep 0.015
    i=$((i + 1))
  done
}

scroll_ticks() {
  [ "$#" -eq 1 ] || usage
  ticks="$1"
  case "$ticks" in
    ''|*[!0-9-]*) usage ;;
  esac
  if [ "$ticks" -ge 0 ]; then
    button=4
    count="$ticks"
  else
    button=5
    count=$((0 - ticks))
  fi
  i=0
  while [ "$i" -lt "$count" ]; do
    xdotool click "$button"
    sleep 0.05
    i=$((i + 1))
  done
}

need_xdotool
cmd="${1:-}"
shift || true

case "$cmd" in
  move)
    move_mouse "$@"
    ;;
  click)
    [ "$#" -ge 2 ] || usage
    x="$1"; y="$2"; button="${3:-1}"
    move_mouse "$x" "$y"
    xdotool click "$button"
    ;;
  type)
    [ "$#" -ge 1 ] || usage
    xdotool type --clearmodifiers --delay "$DELAY" "$*"
    ;;
  key)
    [ "$#" -ge 1 ] || usage
    xdotool key --clearmodifiers "$@"
    ;;
  scroll)
    scroll_ticks "$@"
    ;;
  *)
    usage
    ;;
esac
