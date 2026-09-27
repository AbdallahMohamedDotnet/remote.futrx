package versiontelemetry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingReporter struct {
	mu       sync.Mutex
	versions []string
	errors   []error
	calls    chan struct{}
}

func (r *recordingReporter) ReportVersion(_ context.Context, version string) error {
	r.mu.Lock()
	r.versions = append(r.versions, version)
	var err error
	if len(r.errors) > 0 {
		err = r.errors[0]
		r.errors = r.errors[1:]
	}
	r.mu.Unlock()
	r.calls <- struct{}{}
	return err
}

type controlledWait struct {
	durations chan time.Duration
	advance   chan struct{}
}

func newControlledWait() *controlledWait {
	return &controlledWait{
		durations: make(chan time.Duration, 3),
		advance:   make(chan struct{}, 3),
	}
}

func (w *controlledWait) wait(ctx context.Context, duration time.Duration) bool {
	w.durations <- duration
	select {
	case <-ctx.Done():
		return false
	case <-w.advance:
		return true
	}
}

type reporterFunc func(context.Context, string) error

func (f reporterFunc) ReportVersion(ctx context.Context, version string) error {
	return f(ctx, version)
}

func TestRunWaitsOneWeekAfterFailureAndSuccess(t *testing.T) {
	recorder := &recordingReporter{
		errors: []error{errors.New("telemetry unavailable")},
		calls:  make(chan struct{}, 3),
	}
	waits := newControlledWait()
	service := newService("42989fe", recorder, waits.wait)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.run(ctx)
		close(done)
	}()

	waitForCall(t, recorder.calls)
	waitForDuration(t, waits.durations, reportInterval)
	waits.advance <- struct{}{}
	waitForCall(t, recorder.calls)
	waitForDuration(t, waits.durations, reportInterval)
	cancel()
	waitForDone(t, done)

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.versions) != 2 {
		t.Fatalf("reported versions = %v, want two calls", recorder.versions)
	}
	for _, version := range recorder.versions {
		if version != "42989fe" {
			t.Fatalf("reported version = %q, want raw version 42989fe", version)
		}
	}
}

func TestStartIsIdempotentAndCancellable(t *testing.T) {
	calls := make(chan struct{}, 2)
	service := New("0.21.0", reporterFunc(func(context.Context, string) error {
		calls <- struct{}{}
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())

	service.Start(ctx)
	service.Start(ctx)
	waitForCall(t, calls)
	cancel()
	select {
	case <-calls:
		t.Fatal("a second Start launched another reporting loop")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRunWithCancelledContextDoesNotReport(t *testing.T) {
	calls := make(chan struct{}, 1)
	service := New("0.21.0", reporterFunc(func(context.Context, string) error {
		calls <- struct{}{}
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.run(ctx)
	select {
	case <-calls:
		t.Fatal("cancelled service reported a version")
	default:
	}
}

func TestAutomaticReportingVersionPolicy(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "dev", want: false},
		{version: "qa-local-42989fe-clean-20260927", want: false},
		{version: "qa-local-", want: false},
		{version: "qa-42989fe", want: false},
		{version: "qa-release-candidate", want: false},
		{version: "qa-local", want: false},
		{version: "qa", want: true},
		{version: "qatest", want: true},
		{version: "Dev", want: true},
		{version: "dev-dirty", want: true},
		{version: "42989fe", want: true},
		{version: "0.20.1-3-g42989fe", want: true},
		{version: "0.21.0", want: true},
	}
	for _, test := range tests {
		if got := shouldReportVersion(test.version); got != test.want {
			t.Errorf("shouldReportVersion(%q) = %v, want %v", test.version, got, test.want)
		}
	}
}

func TestStartSkipsDevelopmentBuilds(t *testing.T) {
	for _, version := range []string{"dev", "qa-local-42989fe-clean-20260927", "qa-42989fe"} {
		t.Run(version, func(t *testing.T) {
			calls := make(chan struct{}, 1)
			service := New(version, reporterFunc(func(context.Context, string) error {
				calls <- struct{}{}
				return nil
			}))
			service.Start(context.Background())
			select {
			case <-calls:
				t.Fatalf("development version %q was reported", version)
			case <-time.After(20 * time.Millisecond):
			}
		})
	}
}

func waitForCall(t *testing.T, calls <-chan struct{}) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for telemetry report")
	}
}

func waitForDuration(t *testing.T, durations <-chan time.Duration, want time.Duration) {
	t.Helper()
	select {
	case got := <-durations:
		if got != want {
			t.Fatalf("next report delay = %s, want %s", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s report delay", want)
	}
}

func waitForDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for telemetry loop to stop")
	}
}
