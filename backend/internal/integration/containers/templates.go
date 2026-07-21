package containers

// Container provisioning: ships shared agent instructions to every target
// declared by the configured profiles.

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

var agentInstructionsTemplate = provisioning.InstructionsTemplate()

// EnsureAgentInstructions pushes the shared system-instructions template to
// all configured targets, grouped by hash marker. Idempotent.
func (c *Client) EnsureAgentInstructions(ctx context.Context, containerName string) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	targets := configuredInstructionTargets(c.AgentProfiles())
	if len(targets) == 0 {
		return nil
	}

	dctx, cancelD := context.WithTimeout(ctx, 30*time.Second)
	defer cancelD()
	created := map[string]bool{}
	for _, target := range targets {
		directory := path.Dir(target.Path)
		if created[directory] {
			continue
		}
		if out, err := c.lxc.Run(dctx, "exec", containerName, "--",
			"install", "-d", "-m", "700", directory); err != nil {
			return fmt.Errorf("mkdir %s: %w; output: %s", directory, err, out)
		}
		created[directory] = true
	}

	type batch struct {
		hashPath string
		paths    []string
	}
	var batches []batch
	for _, target := range targets {
		index := -1
		for i := range batches {
			if batches[i].hashPath == target.HashPath {
				index = i
				break
			}
		}
		if index < 0 {
			batches = append(batches, batch{hashPath: target.HashPath})
			index = len(batches) - 1
		}
		batches[index].paths = append(batches[index].paths, target.Path)
	}
	for _, batch := range batches {
		if err := c.templates.push(ctx, containerName, agentInstructionsTemplate,
			batch.hashPath, "644", batch.paths...); err != nil {
			return err
		}
	}
	return nil
}

func configuredInstructionTargets(profiles []provisioning.Profile) []provisioning.InstructionTarget {
	targets := make([]provisioning.InstructionTarget, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Instructions != nil {
			targets = append(targets, *profile.Instructions)
		}
	}
	return targets
}
