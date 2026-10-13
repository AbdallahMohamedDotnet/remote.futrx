package applications

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackendBuildHonorsConfiguredDeadline(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "slow-go")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexec sleep 60\n"), 0755); err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(root, tool, 10*time.Millisecond)
	plan := buildPlan{
		applicationID: "slow", fingerprint: "test", binary: filepath.Join(builder.binaryDir(), "slow-test"),
		backendModule: "module example.test/slow\n", sdkModule: "module example.test/sdk\n", entry: ".",
	}
	started := time.Now()
	err := builder.compile(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "gave up after 10ms") {
		t.Fatalf("build error=%v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("configured compile deadline did not stop the toolchain")
	}
	if _, err := os.Stat(plan.binary); !os.IsNotExist(err) {
		t.Fatal("a cancelled build installed a binary")
	}
}
