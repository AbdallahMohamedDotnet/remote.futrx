#!/usr/bin/env bash
# Shared by infra/lib/node-toolchain.sh and the embedded workspace image recipe.

node_version_satisfies() {
    local major="$1" minimum="$2" current
    current="$(node -p 'process.versions.node' 2>/dev/null)" || return 1
    [[ "$current" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    [ "${current%%.*}" = "$major" ] \
        && dpkg --compare-versions "$current" ge "$minimum"
}

require_node_version() {
    if ! node_version_satisfies "$1" "$2"; then
        printf 'Node %s.x >=%s is required; installed runtime is missing, incompatible, or below the required version\n' "$1" "$2" >&2
        return 1
    fi
}

# install_nodesource_node is supplied by infra/lib/host-apt.sh.
ensure_node_toolchain() {
    local major="$1" minimum="$2"
    if ! node_version_satisfies "$major" "$minimum"; then
        printf 'Installing Node %s.x (>=%s) from NodeSource\n' "$major" "$minimum"
        install_nodesource_node "$major" || return 1
    fi
    # Repository lag or a Node earlier on PATH must not report a successful update.
    require_node_version "$major" "$minimum"
}
