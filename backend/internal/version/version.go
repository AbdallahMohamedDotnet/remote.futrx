// Package version carries the build-time identity of the running binary.
package version

// Version is stamped by the build via
//
//	-ldflags "-X github.com/futrx-com/remote.futrx.com/internal/version.Version=<git describe>"
//
// and stays "dev" for ad-hoc `go build` / `go run` invocations.
var Version = "dev"
