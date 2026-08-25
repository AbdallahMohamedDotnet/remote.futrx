// install-host-agents converges every local host CLI declared by the compiled
// agent module catalog. It is run by the infrastructure installer after Node
// and Go are available.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/futrx-com/remote.futrx.com/internal/agent/builtin"
	hostcliruntime "github.com/futrx-com/remote.futrx.com/internal/integration/hostcli"
	"github.com/futrx-com/remote.futrx.com/internal/service/agent/hostcli"
)

func main() {
	plan := flag.Bool("plan", false, "print the catalog-derived host CLI plan without changing the host")
	flag.Parse()
	log.SetFlags(0)

	catalog, err := builtin.Catalog()
	if err != nil {
		log.Fatalf("configure agent modules: %v", err)
	}
	profiles := catalog.HostProfiles()
	if *plan {
		for _, profile := range profiles {
			packageName := profile.CLI.PackageName
			if packageName == "" {
				packageName = "-"
			}
			fmt.Printf("%s\t%s\t%s\t%s\t%s\n", profile.ID, profile.CLI.Binary, profile.CLI.Version, profile.CLI.InstallMode, packageName)
		}
		return
	}

	results, err := hostcli.New(hostcliruntime.New()).EnsureAll(context.Background(), profiles)
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		state := "already current"
		if result.Changed {
			state = "installed"
		}
		if !result.VersionChecked {
			fmt.Printf("%s: %s (version not checked)\n", result.Name, state)
			continue
		}
		if result.DetectedVersion == "" {
			fmt.Printf("%s: %s (version not detected)\n", result.Name, state)
			continue
		}
		fmt.Printf("%s %s: %s\n", result.Name, result.DetectedVersion, state)
	}
}
