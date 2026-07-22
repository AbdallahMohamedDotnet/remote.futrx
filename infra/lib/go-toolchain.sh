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

go_toolchain_official_urls() {
    local version="$1" go_arch="$2"
    local filename="go${version}.linux-${go_arch}.tar.gz"
    printf '%s\n' \
        "https://dl.google.com/go/${filename}" \
        "https://go.dev/dl/${filename}"
}

fetch_go_toolchain_github_url() {
    local version="$1" go_arch="$2" actions_arch=""
    case "$go_arch" in
        amd64) actions_arch=x64 ;;
        arm64) actions_arch=arm64 ;;
        *) return 1 ;;
    esac

    curl --fail --silent --show-error --location \
        --connect-timeout 20 --retry 3 --retry-delay 2 \
        'https://raw.githubusercontent.com/actions/go-versions/main/versions-manifest.json' \
        | jq -r --arg version "$version" --arg arch "$actions_arch" '
            .[]
            | select(.version == $version)
            | .files[]
            | select(.platform == "linux" and .arch == $arch)
            | .download_url
        ' \
        | head -1
}

download_go_toolchain_archive() {
    local archive="$1" url="$2"
    rm -f "$archive"
    if ! curl --fail --silent --show-error --location \
        --connect-timeout 20 --retry 3 --retry-delay 2 \
        -o "$archive" "$url"; then
        return 1
    fi
    if [ ! -s "$archive" ] || ! tar -tzf "$archive" >/dev/null 2>&1; then
        return 2
    fi
}

stage_go_toolchain_archive() {
    local archive="$1" stage="$2" source_url="$3" desired="$4"
    local unpack="$stage/unpack"
    mkdir -p "$unpack"
    if ! tar -C "$unpack" -xzf "$archive"; then
        rm -f "$archive"
        rm -rf "$stage"
        err "Could not extract Go archive downloaded from $source_url"
        return 1
    fi
    rm -f "$archive"

    # go.dev archives contain go/bin/go; GitHub Actions archives contain
    # bin/go at their root. Normalize both into stage/ready before replacing
    # anything on the host.
    local payload=""
    if [ -x "$unpack/go/bin/go" ]; then
        payload="$unpack/go"
    elif [ -x "$unpack/bin/go" ]; then
        payload="$unpack"
    else
        rm -rf "$stage"
        err "Downloaded archive does not contain a Go toolchain."
        return 1
    fi
    if ! mv "$payload" "$stage/ready"; then
        rm -rf "$stage"
        err "Could not stage the downloaded Go toolchain."
        return 1
    fi
    if [ -d "$unpack" ]; then
        rmdir "$unpack" 2>/dev/null || true
    fi

    local staged_version
    staged_version="$(go_toolchain_version "$stage/ready/bin/go")"
    if [ "$staged_version" != "$desired" ]; then
        rm -rf "$stage"
        err "Downloaded Go version ${staged_version:-unknown}; expected $desired"
        return 1
    fi
}

restore_go_toolchain_backup() {
    local backup="$1" install_dir="$2"
    local backup_dir="$backup/go"

    if [ ! -e "$backup_dir" ] && [ ! -L "$backup_dir" ]; then
        err "Could not restore the previous Go toolchain; expected backup at $backup_dir"
        return 1
    fi
    if [ -e "$install_dir" ] || [ -L "$install_dir" ]; then
        err "Could not restore the previous Go toolchain because $install_dir still exists; backup preserved at $backup_dir"
        return 1
    fi
    if ! mv "$backup_dir" "$install_dir"; then
        err "Could not restore the previous Go toolchain; backup preserved at $backup_dir"
        return 1
    fi
    if ! rmdir "$backup" 2>/dev/null; then
        warn "Restored the previous Go toolchain, but could not remove backup directory $backup"
    fi
    return 0
}

install_staged_go_toolchain() {
    local stage="$1" install_root="$2" install_dir="$3" desired="$4"
    local backup=""

    # Stage and verify before replacing an existing Go directory. If the
    # final move or verification fails, restore the previous toolchain so an
    # update can never leave an existing server without a working Go binary.
    if [ -e "$install_dir" ] || [ -L "$install_dir" ]; then
        backup="$(mktemp -d "${install_root}/.go-backup.XXXXXX")"
        if ! mv "$install_dir" "$backup/go"; then
            rm -rf "$backup" "$stage"
            err "Could not stage the existing Go installation for replacement."
            return 1
        fi
    fi

    if ! mv "$stage/ready" "$install_dir"; then
        if [ -n "$backup" ]; then
            if ! restore_go_toolchain_backup "$backup" "$install_dir"; then
                err "Could not install Go into $install_dir and could not restore the previous toolchain."
            fi
        fi
        rm -rf "$stage"
        err "Could not install Go into $install_dir"
        return 1
    fi
    rmdir "$stage" 2>/dev/null || rm -rf "$stage"

    local installed_version
    installed_version="$(go_toolchain_version "$install_dir/bin/go")"
    if [ "$installed_version" != "$desired" ]; then
        if ! rm -rf "$install_dir"; then
            if [ -n "$backup" ]; then
                err "Go verification failed and the invalid installation could not be removed; backup preserved at $backup/go"
            else
                err "Go verification failed and the invalid installation could not be removed."
            fi
            return 1
        fi
        if [ -n "$backup" ]; then
            if restore_go_toolchain_backup "$backup" "$install_dir"; then
                err "Go verification failed after install; restored the previous toolchain."
            else
                err "Go verification failed after install and the previous toolchain could not be restored."
            fi
        else
            err "Go verification failed after install."
        fi
        return 1
    fi

    if [ -n "$backup" ]; then
        rm -rf "$backup"
    fi
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

    local deb_arch go_arch
    deb_arch="$(dpkg --print-architecture)"
    if ! go_arch="$(go_toolchain_arch "$deb_arch")"; then
        err "Unsupported CPU architecture for Go: $deb_arch"
        return 1
    fi
    # Use the direct official download host first. go.dev normally redirects
    # there, but some fresh-server networks have returned a transient 404 for
    # the redirect endpoint. The canonical go.dev URL remains a fallback.
    local official_urls urls=() url
    official_urls="$(go_toolchain_official_urls "$desired" "$go_arch")"
    while IFS= read -r url; do
        [ -n "$url" ] && urls+=("$url")
    done <<< "$official_urls"

    local archive stage downloaded="" download_status
    archive="$(mktemp --suffix=.tgz)"
    mkdir -p "$install_root" "$bin_dir"
    stage="$(mktemp -d "${install_root}/.go-stage.XXXXXX")"

    for url in "${urls[@]}"; do
        download_status=0
        download_go_toolchain_archive "$archive" "$url" || download_status=$?
        case "$download_status" in
            0) downloaded="$url"; break ;;
            1) warn "Go download failed from $url; trying the next official source." ;;
            2) warn "Downloaded an invalid Go archive from $url; trying the next official source." ;;
        esac
    done

    # GitHub Actions maintains verified Go distributions for its runners.
    # This gives servers whose network blocks Google download hosts a fully
    # automatic third path through GitHub, which the installer already needs
    # to reach for the application repository.
    if [ -z "$downloaded" ]; then
        local github_url=""
        github_url="$(fetch_go_toolchain_github_url "$desired" "$go_arch" || true)"
        if [ -n "$github_url" ]; then
            urls+=("$github_url")
            download_status=0
            download_go_toolchain_archive "$archive" "$github_url" || download_status=$?
            case "$download_status" in
                0) downloaded="$github_url" ;;
                1) warn "Go download failed from GitHub." ;;
                2) warn "Downloaded an invalid Go archive from GitHub." ;;
            esac
        else
            warn "No GitHub-hosted Go ${desired} archive is available for ${go_arch}."
        fi
    fi

    if [ -z "$downloaded" ]; then
        rm -f "$archive"
        rm -rf "$stage"
        err "Could not download Go ${desired} for Debian ${deb_arch} (Go ${go_arch})."
        err "Tried: ${urls[*]}"
        return 1
    fi

    stage_go_toolchain_archive "$archive" "$stage" "$downloaded" "$desired" || return 1
    install_staged_go_toolchain "$stage" "$install_root" "$install_dir" "$desired" || return 1
    ln -sf "$install_dir/bin/go" "$bin_dir/go"
    ln -sf "$install_dir/bin/gofmt" "$bin_dir/gofmt"
    hash -r

    if dpkg -s golang-go >/dev/null 2>&1; then
        warn "distro golang-go remains installed but is shadowed by /usr/local/go"
    fi
    ok "$("$install_dir/bin/go" version)"
}
