package project

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
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

func TestStartRestartsFrozenContainer(t *testing.T) {
	repo := &startTestRepository{meta: Meta{
		ID:            ID("abcd"),
		Name:          "project",
		ContainerName: "project",
		Status:        StatusRunning,
	}}
	lifecycle := &startTestLifecycle{state: ContainerStateFrozen}
	service := New(repo, ContainerDependencies{Lifecycle: lifecycle}, nil, nil)

	got, err := service.Start(context.Background(), repo.meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRunning {
		t.Fatalf("Start() status = %q, want %q", got.Status, StatusRunning)
	}

	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.restartCalls != 1 {
		t.Fatalf("Restart() calls = %d, want 1", lifecycle.restartCalls)
	}
	if lifecycle.startCalls != 0 {
		t.Fatalf("Start() calls = %d, want 0", lifecycle.startCalls)
	}
	if lifecycle.state != ContainerStateRunning {
		t.Fatalf("container state = %q, want %q", lifecycle.state, ContainerStateRunning)
	}
}

func TestRunStateTransitionsWaitForConcurrentStart(t *testing.T) {
	tests := []struct {
		name      string
		wantCall  string
		operation func(context.Context, *Service, ID) error
	}{
		{
			name:     "stop",
			wantCall: "stop",
			operation: func(ctx context.Context, service *Service, id ID) error {
				_, err := service.Stop(ctx, id)
				return err
			},
		},
		{
			name:     "restart",
			wantCall: "restart",
			operation: func(ctx context.Context, service *Service, id ID) error {
				_, err := service.Restart(ctx, id)
				return err
			},
		},
		{
			name:     "delete",
			wantCall: "delete",
			operation: func(ctx context.Context, service *Service, id ID) error {
				return service.Delete(ctx, id)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &startTestRepository{meta: Meta{
				ID:            ID("abcd"),
				Name:          "project",
				ContainerName: "project",
				Status:        StatusRunning,
			}}
			launchStarted := make(chan struct{})
			releaseLaunch := make(chan struct{})
			transitionCalls := make(chan string, 1)
			lifecycle := &startTestLifecycle{
				state:           ContainerStateMissing,
				launchStarted:   launchStarted,
				releaseLaunch:   releaseLaunch,
				transitionCalls: transitionCalls,
			}
			service := New(repo, ContainerDependencies{Lifecycle: lifecycle}, nil, nil)

			startResult := make(chan error, 1)
			go func() {
				_, err := service.Start(context.Background(), repo.meta.ID)
				startResult <- err
			}()
			<-launchStarted

			operationStarted := make(chan struct{})
			operationResult := make(chan error, 1)
			go func() {
				close(operationStarted)
				operationResult <- test.operation(context.Background(), service, repo.meta.ID)
			}()
			<-operationStarted
			select {
			case call := <-transitionCalls:
				close(releaseLaunch)
				<-startResult
				<-operationResult
				t.Fatalf("%s reached lifecycle before concurrent launch completed", call)
			case <-time.After(50 * time.Millisecond):
			}

			close(releaseLaunch)
			if err := <-startResult; err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			if err := <-operationResult; err != nil {
				t.Fatalf("%s error: %v", test.name, err)
			}
			if call := <-transitionCalls; call != test.wantCall {
				t.Fatalf("lifecycle call = %q, want %q", call, test.wantCall)
			}
		})
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
	mu              sync.Mutex
	state           ContainerState
	launchErr       error
	launchCalls     int
	startCalls      int
	restartCalls    int
	launchStarted   chan struct{}
	releaseLaunch   <-chan struct{}
	transitionCalls chan<- string
}

func (l *startTestLifecycle) Available() bool { return true }

func (l *startTestLifecycle) State(context.Context, string) (ContainerState, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state, nil
}

func (l *startTestLifecycle) Launch(context.Context, Meta) error {
	l.mu.Lock()
	l.launchCalls++
	launchErr := l.launchErr
	launchStarted := l.launchStarted
	releaseLaunch := l.releaseLaunch
	l.mu.Unlock()

	if launchStarted != nil {
		close(launchStarted)
	}
	if releaseLaunch != nil {
		<-releaseLaunch
	}
	if launchErr != nil {
		return launchErr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = ContainerStateRunning
	return nil
}

func (l *startTestLifecycle) Start(context.Context, string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.startCalls++
	l.state = ContainerStateRunning
	return nil
}

func (l *startTestLifecycle) Stop(context.Context, string) error {
	l.recordTransition("stop", ContainerStateStopped)
	return nil
}
func (l *startTestLifecycle) Restart(context.Context, string) error {
	l.recordTransition("restart", ContainerStateRunning)
	return nil
}
func (l *startTestLifecycle) Delete(context.Context, string) error {
	l.recordTransition("delete", ContainerStateMissing)
	return nil
}

func (l *startTestLifecycle) recordTransition(name string, state ContainerState) {
	l.mu.Lock()
	if name == "restart" {
		l.restartCalls++
	}
	l.state = state
	transitionCalls := l.transitionCalls
	l.mu.Unlock()
	if transitionCalls != nil {
		transitionCalls <- name
	}
}

func (l *startTestLifecycle) EnsureResources(context.Context, string) error { return nil }
func (l *startTestLifecycle) SetResourceLimits(context.Context, string, ContainerLimits) error {
	return nil
}
