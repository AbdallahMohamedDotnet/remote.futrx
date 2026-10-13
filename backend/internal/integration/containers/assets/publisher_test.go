package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type publicationRunner struct {
	minimumBudget   time.Duration
	blockWrite      bool
	writes, markers int
}

func (*publicationRunner) Available() bool { return true }

func (r *publicationRunner) Run(ctx context.Context, args ...string) (string, error) {
	if len(args) > 1 && args[0] == "file" && args[1] == "push" {
		r.writes++
		if r.blockWrite {
			<-ctx.Done()
			return "", errors.New("signal: killed")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < r.minimumBudget {
			return "", context.DeadlineExceeded
		}
		return "", nil
	}
	return "", fmt.Errorf("marker missing")
}

func (r *publicationRunner) RunStdin(ctx context.Context, _ io.Reader, _ ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	r.markers++
	return "", nil
}

func TestApplicationPublicationAllowsSlowCopies(t *testing.T) {
	runner := &publicationRunner{minimumBudget: 45 * time.Second}
	publisher := NewPublisher(runner, Timeouts{Query: 2 * time.Minute, Write: 2 * time.Minute})
	if err := publisher.PushVerified(context.Background(), "project", []byte("content"), "/marker", "600", "/destination"); err != nil {
		t.Fatalf("copy needing more than the old 30s budget failed: %v", err)
	}
	if runner.writes != 1 || runner.markers != 1 {
		t.Fatalf("publication=%d copies/%d markers", runner.writes, runner.markers)
	}
}

func TestPublicationDeadlineDoesNotCommitMarker(t *testing.T) {
	runner := &publicationRunner{blockWrite: true}
	publisher := NewPublisher(runner, Timeouts{Write: time.Millisecond})
	err := publisher.Push(context.Background(), "project", []byte("content"), "/marker", "600", "/destination")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "limit 1ms") {
		t.Fatalf("deadline error=%v", err)
	}
	if runner.markers != 0 {
		t.Fatal("a failed copy committed its hash marker")
	}
}

func TestWorkspacePublicationKeepsExistingBudget(t *testing.T) {
	runner := &publicationRunner{minimumBudget: 45 * time.Second}
	err := NewPublisher(runner).Push(context.Background(), "project", []byte("content"), "/marker", "600", "/destination")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("default copy unexpectedly gained an application budget: %v", err)
	}
}
