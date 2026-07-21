#!/usr/bin/env bash
# Install an exact Go toolchain safely on Debian/Ubuntu hosts.
#
# This file is sourced by infra/steps/01-host-deps.sh and expects the caller's
# log, warn, ok, and err helpers. Keeping the download/install logic isolated
# also lets us exercise fresh-install and upgrade paths in disposable Linux
# containers without running the rest of the server installer.

go_toolchain_arch() {
    case "$1" in
        amd64|arm64|riscv64|s390x) printf '%s\n' "$1" ;;
        i386)                      printf '%s\n' "386" ;;
        armhf)                     printf '%s\n' "armv6l" ;;
        ppc64el)                   printf '%s\n' "ppc64le" ;;
        *)                         return 1 ;;
    esac
}

go_toolchain_version() {
    local binary="${1:-}"
    if [ -z "$binary" ] || [ ! -x "$binary" ]; then
        return 0
    fi
    "$binary" version 2>/dev/null \
        | grep -Eo 'go[0-9]+(\.[0-9]+)+' \
        | head -1 \
        | sed 's/^go//' || true
}

ensure_go_toolchain() {
    local desired="$1"
    local install_root="${GO_INSTALL_ROOT:-/usr/local}"
    local install_dir="$install_root/go"
    local bin_dir="$install_root/bin"
    local current_binary="" current=""
    current_binary="$(command -v go 2>/dev/null || true)"
    current="$(go_toolchain_version "$current_binary")"

    if [ "$current" = "$desired" ]; then
        ok "$($current_binary version)"
        return 0
    fi

    log "Installing Go ${desired} (was ${current:-missing})"

    local deb_arch go_arch filename
    deb_arch="$(dpkg --print-architecture)"
    if ! go_arch="$(go_toolchain_arch "$deb_arch")"; then
        err "Unsupported CPU architecture for Go: $deb_arch"
        return 1
    fi
    filename="go${desired}.linux-${go_arch}.tar.gz"

    # Use the direct official download host first. go.dev normally redirects
    # there, but some fresh-server networks have returned a transient 404 for
    # the redirect endpoint. The canonical go.dev URL remains a fallback.
    local urls=(
        "https://dl.google.com/go/${filename}"
        "https://go.dev/dl/${filename}"
    )
    local archive stage url downloaded=""
    archive="$(mktemp --suffix=.tgz)"
    mkdir -p "$install_root" "$bin_dir"
    stage="$(mktemp -d "${install_root}/.go-stage.XXXXXX")"

    for url in "${urls[@]}"; do
        rm -f "$archive"
        if curl --fail --silent --show-error --location \
            --connect-timeout 20 --retry 3 --retry-delay 2 \
            -o "$archive" "$url"; then
            if [ -s "$archive" ] && tar -tzf "$archive" >/dev/null 2>&1; then
                downloaded="$url"
                break
            fi
            warn "Downloaded an invalid Go archive from $url; trying the next official source."
        else
            warn "Go download failed from $url; trying the next official source."
        fi
    done

    if [ -z "$downloaded" ]; then
        rm -f "$archive"
        rm -rf "$stage"
        err "Could not download Go ${desired} for Debian ${deb_arch} (Go ${go_arch})."
        err "Tried: ${urls[*]}"
        return 1
    fi

    if ! tar -C "$stage" -xzf "$archive"; then
        rm -f "$archive"
        rm -rf "$stage"
        err "Could not extract Go archive downloaded from $downloaded"
        return 1
    fi
    rm -f "$archive"

    local staged_version
    staged_version="$(go_toolchain_version "$stage/go/bin/go")"
    if [ "$staged_version" != "$desired" ]; then
        rm -rf "$stage"
        err "Downloaded Go version ${staged_version:-unknown}; expected $desired"
        return 1
    fi

    # Stage and verify before replacing an existing Go directory. If the
    # final move or verification fails, restore the previous toolchain so an
    # update can never leave an existing server without a working Go binary.
    local backup=""
    if [ -e "$install_dir" ] || [ -L "$install_dir" ]; then
        backup="$(mktemp -d "${install_root}/.go-backup.XXXXXX")"
        if ! mv "$install_dir" "$backup/go"; then
            rm -rf "$backup" "$stage"
            err "Could not stage the existing Go installation for replacement."
            return 1
        fi
    fi

    if ! mv "$stage/go" "$install_dir"; then
        if [ -n "$backup" ] && [ -e "$backup/go" ]; then
            mv "$backup/go" "$install_dir" || true
        fi
        if [ -n "$backup" ]; then
            rm -rf "$backup"
        fi
        rm -rf "$stage"
        err "Could not install Go into $install_dir"
        return 1
    fi
    rmdir "$stage" 2>/dev/null || rm -rf "$stage"

    local installed_version
    installed_version="$(go_toolchain_version "$install_dir/bin/go")"
    if [ "$installed_version" != "$desired" ]; then
        rm -rf "$install_dir"
        if [ -n "$backup" ] && [ -e "$backup/go" ]; then
            mv "$backup/go" "$install_dir" || true
        fi
        if [ -n "$backup" ]; then
            rm -rf "$backup"
        fi
        err "Go verification failed after install; restored the previous toolchain."
        return 1
    fi

    if [ -n "$backup" ]; then
        rm -rf "$backup"
    fi
    ln -sf "$install_dir/bin/go" "$bin_dir/go"
    ln -sf "$install_dir/bin/gofmt" "$bin_dir/gofmt"
    hash -r

    if dpkg -s golang-go >/dev/null 2>&1; then
        warn "distro golang-go remains installed but is shadowed by /usr/local/go"
    fi
    ok "$("$install_dir/bin/go" version)"
}
