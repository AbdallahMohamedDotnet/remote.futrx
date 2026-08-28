package kimi

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestRunRejectsPlanBeforeLaunchingPrintTransport(t *testing.T) {
	err := (&Provider{}).Run(context.Background(), agent.RunRequest{Mode: agent.RunModePlan}, nil)
	if !errors.Is(err, agent.ErrUnsupportedRunMode) {
		t.Fatalf("run error = %v", err)
	}
}

func TestArgsDoNotAddPlanBecausePrintTransportIsIncompatible(t *testing.T) {
	provider := &Provider{}
	plan := provider.args(agent.RunRequest{Prompt: "inspect", Mode: agent.RunModePlan})
	if slices.Contains(plan, "--plan") {
		t.Fatalf("print transport must not combine -p with --plan: %#v", plan)
	}

	defaults := provider.args(agent.RunRequest{Prompt: "implement", Mode: agent.RunModeDefault})
	if slices.Contains(defaults, "--plan") {
		t.Fatalf("default mode unexpectedly enabled Plan: %#v", defaults)
	}
}

func TestArgsPreserveExactConfiguredModelAlias(t *testing.T) {
	provider := &Provider{}
	args := provider.args(agent.RunRequest{Prompt: "inspect", Model: "moonshot/kimi-k2[1m]"})
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--model" && args[index+1] == "moonshot/kimi-k2[1m]" {
			return
		}
	}
	t.Fatalf("exact model alias missing: %#v", args)
}
