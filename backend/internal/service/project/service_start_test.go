package project

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestStartPersistsAndReturnsLaunchError(t *testing.T) {
	wantErr := errors.New("prepare workspace: dangling browser lock")
	repo := &startTestRepository{meta: Meta{
		ID:            ID("abcd"),
		Name:          "project",
		ContainerName: "project",
		Status:        StatusMissing,
	}}
	lifecycle := &startTestLifecycle{state: ContainerStateMissing, launchErr: wantErr}
	service := New(repo, ContainerDependencies{Lifecycle: lifecycle}, nil, nil)

	got, err := service.Start(context.Background(), repo.meta.ID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Start() error = %v, want %v", err, wantErr)
	}
	if got.Status != StatusError || got.ErrorMsg != wantErr.Error() {
		t.Fatalf("Start() meta = %#v, want persisted launch error", got)
	}
	if repo.meta.Status != StatusError || repo.meta.ErrorMsg != wantErr.Error() {
		t.Fatalf("repository meta = %#v, want persisted launch error", repo.meta)
	}
}

func TestConcurrentStartLaunchesMissingContainerOnce(t *testing.T) {
	repo := &startTestRepository{meta: Meta{
		ID:            ID("abcd"),
		Name:          "project",
		ContainerName: "project",
		Status:        StatusRunning,
	}}
	lifecycle := &startTestLifecycle{state: ContainerStateMissing}
	service := New(repo, ContainerDependencies{Lifecycle: lifecycle}, nil, nil)

	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := service.Start(context.Background(), repo.meta.ID)
			errs <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.launchCalls != 1 {
		t.Fatalf("Launch() calls = %d, want 1", lifecycle.launchCalls)
	}
}

type startTestRepository struct {
	mu   sync.Mutex
	meta Meta
}

func (r *startTestRepository) List(context.Context) ([]Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return []Meta{r.meta}, nil
}

func (r *startTestRepository) Create(_ context.Context, meta Meta) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.meta = meta
	return meta, nil
}

func (r *startTestRepository) Get(context.Context, ID) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.meta, nil
}

func (r *startTestRepository) GetBySlug(context.Context, string) (Meta, error) {
	return r.Get(context.Background(), r.meta.ID)
}

func (r *startTestRepository) Update(_ context.Context, _ ID, fn func(*Meta)) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(&r.meta)
	return r.meta, nil
}

func (r *startTestRepository) SetStatus(_ context.Context, _ ID, status Status, errMsg string) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.meta.Status = status
	r.meta.ErrorMsg = errMsg
	return r.meta, nil
}

func (r *startTestRepository) Delete(context.Context, ID) error { return nil }

type startTestLifecycle struct {
	mu          sync.Mutex
	state       ContainerState
	launchErr   error
	launchCalls int
}

func (l *startTestLifecycle) Available() bool { return true }

func (l *startTestLifecycle) State(context.Context, string) (ContainerState, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state, nil
}

func (l *startTestLifecycle) Launch(context.Context, Meta) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.launchCalls++
	if l.launchErr != nil {
		return l.launchErr
	}
	l.state = ContainerStateRunning
	return nil
}

func (l *startTestLifecycle) Start(context.Context, string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = ContainerStateRunning
	return nil
}

func (l *startTestLifecycle) Stop(context.Context, string) error            { return nil }
func (l *startTestLifecycle) Restart(context.Context, string) error         { return nil }
func (l *startTestLifecycle) Delete(context.Context, string) error          { return nil }
func (l *startTestLifecycle) EnsureResources(context.Context, string) error { return nil }
func (l *startTestLifecycle) SetResourceLimits(context.Context, string, ContainerLimits) error {
	return nil
}
