package applications

import (
	"fmt"
	"strings"

	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
)

type goToolchainPin struct {
	archiveSHA256 string
	moduleSHA256  string
}

// Update these with ApplicationContainerGoVersion. Release archive hashes come
// from go.dev/dl/?mode=json&include=all; module ZIP hashes were calculated after
// go mod download verified the toolchain modules against sum.golang.org.
var containerGoPins = map[string]goToolchainPin{
	"amd64": {"da18191ddb7db8a9339816f3e2b54bdded8047cdc2a5d67059478f8d1595c43f", "4c2a63c008dbb90112f22ff9e47eb6fb487b17da1c70ecc7b9fad77577b8031e"},
	"arm64": {"fd2bccce882e29369f56c86487663bb78ba7ea9e02188a5b0269303a0c3d33ab", "67d836d2f0e17ce538757ba43439fa8f77eb00fe9798e0b0920d94925184e7a6"},
}

func containerGoBootstrap(goRoot string, pins map[string]goToolchainPin) string {
	var cases strings.Builder
	for _, arch := range []string{"amd64", "arm64"} {
		pin := pins[arch]
		fmt.Fprintf(&cases, "      %s) ARCHIVE_SHA=%s; MODULE_SHA=%s ;;\n", arch,
			shellQuote(pin.archiveSHA256), shellQuote(pin.moduleSHA256))
	}
	return strings.NewReplacer(
		"__GO_ROOT__", shellQuote(goRoot),
		"__GO_VERSION__", shellQuote(configconstants.ApplicationContainerGoVersion),
		"__PINS__", cases.String(),
	).Replace(containerGoBootstrapTemplate)
}

const containerGoBootstrapTemplate = `
  GOROOT_DIR=__GO_ROOT__
  GO_VERSION=__GO_VERSION__
  if ! "$GOROOT_DIR/bin/go" version 2>/dev/null | grep -q "go${GO_VERSION} "; then
    install_application_go() (
      set -euo pipefail
      ARCH="$(dpkg --print-architecture)"
      case "$ARCH" in
__PINS__
        *) echo "install: unsupported container architecture: $ARCH" >&2; exit 1 ;;
      esac
      if ! command -v curl >/dev/null 2>&1 || ! command -v flock >/dev/null 2>&1; then
        apt-get -o DPkg::Lock::Timeout=300 update -qq
        apt-get -o DPkg::Lock::Timeout=300 install -y -qq --no-install-recommends curl ca-certificates util-linux
      fi
      # Different applications share this compiler. Recheck after taking the
      # lock so concurrent installs never move a toolchain another is using.
      exec 9>"$(dirname "$GOROOT_DIR")/.remote-go-install.lock"
      flock 9
      if "$GOROOT_DIR/bin/go" version 2>/dev/null | grep -q "go${GO_VERSION} "; then return 0; fi
      GO_STAGE="$(mktemp -d "$(dirname "$GOROOT_DIR")/.remote-go.XXXXXX")"
      trap 'rm -rf -- "$GO_STAGE"' EXIT
      for FORMAT in archive module; do
        rm -rf -- "$GO_STAGE/unpack"
        mkdir -p "$GO_STAGE/unpack"
        if [ "$FORMAT" = archive ]; then
          URL="https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
          SHA="$ARCHIVE_SHA"
        else
          URL="https://proxy.golang.org/golang.org/toolchain/@v/v0.0.1-go${GO_VERSION}.linux-${ARCH}.zip"
          SHA="$MODULE_SHA"
          if ! command -v unzip >/dev/null 2>&1; then
            apt-get -o DPkg::Lock::Timeout=300 update -qq
            apt-get -o DPkg::Lock::Timeout=300 install -y -qq --no-install-recommends unzip
          fi
        fi
        printf 'Installing Go %s: downloading from %s\n' "$GO_VERSION" "$URL" >&2
        # Abort stalled transfers, but keep allowing a slow download that is
        # making progress. The application's outer deadline bounds the work.
        if ! curl -fSL --connect-timeout 20 --speed-limit 1024 --speed-time 30 \
          --retry 1 --retry-all-errors --retry-delay 2 --retry-max-time 90 \
          -o "$GO_STAGE/download" "$URL"; then
          echo "Go download failed; trying the next source" >&2
          continue
        fi
        if ! printf '%s  %s\n' "$SHA" "$GO_STAGE/download" | sha256sum --check --status; then
          echo "Go archive checksum mismatch; trying the next source" >&2
          continue
        fi
        if [ "$FORMAT" = archive ]; then
          if ! tar -xzf "$GO_STAGE/download" -C "$GO_STAGE/unpack"; then continue; fi
          READY="$GO_STAGE/unpack/go"
        else
          if ! unzip -q "$GO_STAGE/download" -d "$GO_STAGE/unpack"; then continue; fi
          READY="$GO_STAGE/unpack/golang.org/toolchain@v0.0.1-go${GO_VERSION}.linux-${ARCH}"
          # Module ZIPs normalize file permissions; restore the executables.
          if ! chmod 755 "$READY/bin/go" "$READY/bin/gofmt" "$READY/pkg/tool/linux_${ARCH}/"*; then continue; fi
        fi
        if ! "$READY/bin/go" version | grep -q "go${GO_VERSION} "; then
          echo "Downloaded Go toolchain has the wrong version" >&2
          continue
        fi
        # Keep an existing toolchain until its replacement has been verified.
        if [ -e "$GOROOT_DIR" ]; then mv "$GOROOT_DIR" "$GO_STAGE/previous"; fi
        if ! mv "$READY" "$GOROOT_DIR"; then
          if [ -e "$GO_STAGE/previous" ]; then mv "$GO_STAGE/previous" "$GOROOT_DIR"; fi
          exit 1
        fi
        return 0
      done
      echo "Installing Go failed: all verified download sources were exhausted" >&2
      exit 1
    )
    install_application_go
  fi
`
