package main

import (
	"bytes"
	"log"
	"testing"
	"time"

	serviceimage "github.com/futrx-com/remote.futrx.com/internal/service/container/image"
)

func TestLogBuildProgressReporterPreservesMessages(t *testing.T) {
	var output bytes.Buffer
	reporter := newLogBuildProgressReporter(log.New(&output, "", 0))
	base := serviceimage.Progress{
		Stage:       2,
		StageCount:  6,
		Description: "Installing tools",
		Elapsed:     62 * time.Second,
	}

	for _, state := range []serviceimage.ProgressState{
		serviceimage.ProgressStarted,
		serviceimage.ProgressRunning,
		serviceimage.ProgressSucceeded,
		serviceimage.ProgressFailed,
	} {
		progress := base
		progress.State = state
		reporter.ReportImageBuildProgress(progress)
	}

	const want = "[2/6] Installing tools...\n" +
		"[2/6] Installing tools still running (1m2s elapsed)\n" +
		"[2/6] Installing tools finished in 1m2s\n" +
		"[2/6] Installing tools failed after 1m2s\n"
	if output.String() != want {
		t.Fatalf("progress output = %q, want %q", output.String(), want)
	}
}
