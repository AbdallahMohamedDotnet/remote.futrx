// build-base-image rebuilds the futrx-remote-dev-base LXD image used by
// every project container. Run it after bumping Node, Claude, or any apt
// dependency in the install script.
//
// Usage:
//
//	go run ./cmd/build-base-image                 # build into the default alias
//	go run ./cmd/build-base-image -overwrite      # delete the existing alias first
//	go run ./cmd/build-base-image -alias mybase   # publish under a custom alias
//
// This binary is self-contained: it requires only the `lxc` CLI on PATH and
// network access to apt + npm. Re-running it on a fresh host is the one-shot
// bootstrap for the application's container fleet.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/lxc"
)

func main() {
	alias := flag.String("alias", containers.BaseImageAlias, "image alias to publish under")
	overwrite := flag.Bool("overwrite", false, "delete any existing image at -alias before publishing")
	flag.Parse()

	log.SetFlags(log.Ltime)

	lxcClient := lxc.New()
	if !lxcClient.Available() {
		log.Fatalf("lxc CLI not found on PATH - install LXD on the host first")
	}
	containerClient := containers.New(lxcClient)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if *overwrite {
		log.Printf("removing existing image %q (if any)...", *alias)
		// Best-effort: ignore the error so a missing alias is fine.
		if out, err := lxcClient.Run(ctx, "image", "delete", *alias); err != nil {
			log.Printf("note: image delete returned: %v; output: %s", err, out)
		}
	}

	log.Printf("building %q from %q...", *alias, containers.BaseImageSourceImage)
	log.Printf("(this typically takes 60-120s — apt update + nodejs + npm install -g @anthropic-ai/claude-code)")

	if err := containerClient.BuildBaseImage(ctx, *alias); err != nil {
		log.Fatalf("build failed: %v", err)
	}

	log.Printf("done. published %q. new project containers will launch from this image.", *alias)
}
