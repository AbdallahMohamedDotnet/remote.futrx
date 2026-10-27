package command

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type notifiedOutput chan string

func (w notifiedOutput) Write(data []byte) (int, error) { w <- string(data); return len(data), nil }

func TestCommandPublishesOutputBeforeCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := make(notifiedOutput, 4)
	done := make(chan struct {
		output string
		err    error
	}, 1)
	go func() {
		text, err := CaptureOutput(WithOutput(ctx, output), exec.CommandContext(ctx, "sh", "-c", "printf 'live output\\n'; exec sleep 60"))
		done <- struct {
			output string
			err    error
		}{text, err}
	}()
	select {
	case text := <-output:
		if !strings.Contains(text, "live output") {
			t.Fatalf("live output=%q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("output was buffered until completion")
	}
	cancel()
	select {
	case result := <-done:
		if result.err == nil || !strings.Contains(result.output, "live output") {
			t.Fatalf("result=%+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("command did not stop on cancellation")
	}
}
