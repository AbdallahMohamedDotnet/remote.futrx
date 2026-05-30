package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
)

type ProcessOptions struct {
	Name               string
	LogID              string
	Provider           agent.ProviderID
	ConversationID     string
	StdoutMaxLineBytes int
	StderrMaxLineBytes int
}

func RunProcess(
	ctx context.Context,
	cmd *exec.Cmd,
	parser agent.LineParser,
	emit func(agent.Event),
	opts ProcessOptions,
) error {
	if emit == nil {
		emit = func(agent.Event) {}
	}
	if parser == nil {
		return errors.New("agent parser is required")
	}
	name := opts.Name
	if name == "" {
		name = "agent"
	}
	logID := opts.LogID
	if logID == "" {
		logID = opts.ConversationID
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn %s: %w", name, err)
	}

	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 8192), maxBytes(opts.StderrMaxLineBytes, 1<<20))
		for sc.Scan() {
			log.Printf("%s[%s] stderr: %s", name, logID, sc.Text())
		}
	}()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), maxBytes(opts.StdoutMaxLineBytes, 16<<20))
	runFailed := false
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		events, err := parser.ParseLine(line)
		if err != nil {
			log.Printf("%s[%s] parse: %v line=%s", name, logID, err, truncate(string(line), 200))
			continue
		}
		for _, ev := range events {
			if ev.Type == agent.EventRunFailed {
				runFailed = true
			}
			emit(ev)
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		emit(agent.Event{
			T:              time.Now().UnixMilli(),
			Type:           agent.EventError,
			Provider:       opts.Provider,
			ConversationID: opts.ConversationID,
			Message:        "stdout: " + err.Error(),
		})
	}

	err = cmd.Wait()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	if err != nil && runFailed {
		return agent.ErrRunFailed
	}
	return err
}

func maxBytes(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
