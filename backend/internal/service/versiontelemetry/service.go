// Package versiontelemetry periodically reports the running application
// version without coupling telemetry availability to server startup or the
// self-update workflow.
package versiontelemetry

import (
	"context"
	"strings"
	"sync"
	"time"
)

const reportInterval = 7 * 24 * time.Hour

type waitFunc func(context.Context, time.Duration) bool

type Service struct {
	version  string
	reporter Reporter
	wait     waitFunc

	mu      sync.Mutex
	started bool
}

func New(version string, reporter Reporter) *Service {
	return newService(version, reporter, waitForInterval)
}

func newService(version string, reporter Reporter, wait waitFunc) *Service {
	return &Service{version: version, reporter: reporter, wait: wait}
}

// Start launches one process-lifetime reporting loop. Calls after the first
// are no-ops, so composition and future reconcilers cannot duplicate the
// heartbeat. Reporting failures never escape this worker or trigger an early
// retry; the next attempt uses the same weekly interval.
func (s *Service) Start(ctx context.Context) {
	if !shouldReportVersion(s.version) {
		return
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run(ctx)
}

func (s *Service) run(ctx context.Context) {
	for ctx.Err() == nil {
		_ = s.reporter.ReportVersion(ctx, s.version)
		if !s.wait(ctx, reportInterval) {
			return
		}
	}
}

func waitForInterval(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func shouldReportVersion(version string) bool {
	return version != "dev" && !strings.HasPrefix(version, "qa-")
}
