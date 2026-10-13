package applications

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appapi "github.com/futrx-com/remote.futrx.com/pkg/applications"
)

type memoryInstallRuns struct {
	mu      sync.Mutex
	records map[installRunKey]InstallRun
}

func (s *memoryInstallRuns) Get(_ context.Context, scope Scope, project, app string) (InstallRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.records[installRunKey{scope, project, app}]
	return run, ok, nil
}
func (s *memoryInstallRuns) Put(_ context.Context, run InstallRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		s.records = map[installRunKey]InstallRun{}
	}
	s.records[installRunKey{run.Scope, run.ProjectID, run.ApplicationID}] = run
	return nil
}

type observedTestInstaller struct {
	recordingInstaller
	entered chan context.Context
	result  chan error
	calls   atomic.Int32
}

func (i *observedTestInstaller) Install(ctx context.Context, spec InstallSpec) error {
	i.calls.Add(1)
	ReportInstallProgress(ctx, "download", "Downloading application dependencies")
	_, _ = fmt.Fprint(InstallOutput(ctx), "Download started\npassword=super-")
	i.entered <- ctx
	select {
	case err := <-i.result:
		_, _ = fmt.Fprint(InstallOutput(ctx), "secret\nDownload finished\n")
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func awaitInstallState(t *testing.T, service *Service, req InstallRequest, state string) InstallRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, found, err := service.Installation(context.Background(), req.Scope, req.ProjectID, req.ApplicationID)
		if err != nil {
			t.Fatal(err)
		}
		if found && run.State == state {
			return run
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("installation never became %s", state)
	return InstallRun{}
}

func observedFixture(t *testing.T) (*Service, *observedTestInstaller, *fakeStore, InstallRequest) {
	t.Helper()
	app := serviceApplicationAt("1")
	app.Env = []EnvVar{{Key: "PASSWORD", Secret: true, Required: true}}
	installer := &observedTestInstaller{entered: make(chan context.Context, 2), result: make(chan error, 2)}
	store := &fakeStore{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	service := New(&singleApplicationRegistry{application: app}, store, installer, nil, &countingAllocator{},
		WithInstallRuns(ctx, &memoryInstallRuns{}))
	req := InstallRequest{ApplicationID: app.ID, Scope: ScopeGlobal, Env: map[string]string{"PASSWORD": "super-secret"}}
	return service, installer, store, req
}

func TestObservedInstallReportsLiveOutputAndRequiresManualRetry(t *testing.T) {
	service, installer, store, req := observedFixture(t)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	initial, err := service.StartInstall(requestCtx, req)
	if err != nil || initial.State != "running" {
		t.Fatalf("start=%+v error=%v", initial, err)
	}
	worker := <-installer.entered
	cancelRequest()
	if worker.Err() != nil {
		t.Fatal("closing the HTTP request cancelled the installation")
	}
	live := awaitInstallState(t, service, req, "running")
	if live.Progress.Phase != "download" || !strings.Contains(live.Log, "Download started") || strings.Contains(live.Log, "super-") {
		t.Fatalf("live progress leaked input or arrived too late: %+v", live)
	}
	if _, err := service.StartInstall(context.Background(), req); !errors.Is(err, ErrInstallInProgress) {
		t.Fatalf("duplicate attempt=%v", err)
	}
	installer.result <- errors.New("curl: (56) connection reset, password=super-secret")
	failed := awaitInstallState(t, service, req, "failed")
	if !strings.Contains(failed.Error, "connection reset") || strings.Contains(failed.Error+failed.Log, "super-secret") {
		t.Fatalf("failure=%+v", failed)
	}
	if installer.calls.Load() != 1 {
		t.Fatal("failed download was retried automatically")
	}
	ReportInstallProgress(worker, "late", "Late progress")
	_, _ = fmt.Fprint(InstallOutput(worker), "late output super-secret")
	settled := awaitInstallState(t, service, req, "failed")
	if settled.Log != failed.Log || settled.Progress != failed.Progress {
		t.Fatal("completed attempt accepted late output")
	}
	instances, _ := store.ListGlobal(context.Background())
	if len(instances) != 1 || instances[0].Status != StatusError {
		t.Fatalf("saved failure=%+v", instances)
	}
	retry, err := service.StartInstall(context.Background(), req)
	if err != nil || retry.ID == initial.ID {
		t.Fatalf("retry=%+v error=%v", retry, err)
	}
	<-installer.entered
	installer.result <- nil
	done := awaitInstallState(t, service, req, "succeeded")
	if done.Error != "" || strings.Contains(done.Log, "connection reset") {
		t.Fatalf("retry kept previous error: %+v", done)
	}
	instances, _ = store.ListGlobal(context.Background())
	if len(instances) != 1 || instances[0].Status != StatusRunning {
		t.Fatalf("retry instances=%+v", instances)
	}
}

func TestObservedInstallShutdownPersistsRetryableFailure(t *testing.T) {
	service, installer, store, req := observedFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	service.installRuns.ctx = ctx
	if _, err := service.StartInstall(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	<-installer.entered
	cancel()
	run := awaitInstallState(t, service, req, "failed")
	if strings.Contains(run.Log, "super-") {
		t.Fatalf("cancelled partial secret leaked: %q", run.Log)
	}
	instances, _ := store.ListGlobal(context.Background())
	if len(instances) != 1 || instances[0].Status != StatusError {
		t.Fatalf("cancelled attempt is not retryable: %+v", instances)
	}
}

type blockedInstallHost struct {
	recordingHost
	entered chan struct{}
	release chan struct{}
}

func (h *blockedInstallHost) Ensure(ctx context.Context, _ appapi.Instance) (appapi.Descriptor, error) {
	close(h.entered)
	select {
	case <-h.release:
		return appapi.Descriptor{}, nil
	case <-ctx.Done():
		return appapi.Descriptor{}, ctx.Err()
	}
}

func TestBackendOnlyApplicationStaysInstallingUntilItsBackendIsReady(t *testing.T) {
	app := backendImage(nil)
	host := &blockedInstallHost{entered: make(chan struct{}), release: make(chan struct{})}
	store := &fakeStore{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := New(&singleApplicationRegistry{application: app}, store, nil, nil, nil,
		WithBackendHost(host), WithInstallRuns(ctx, nil))
	req := InstallRequest{ApplicationID: app.ID, Scope: ScopeGlobal}
	if _, err := service.StartInstall(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	<-host.entered
	instances, _ := store.ListGlobal(context.Background())
	if len(instances) != 1 || instances[0].Status != StatusInstalling {
		t.Fatalf("backend exposed before readiness: %+v", instances)
	}
	close(host.release)
	awaitInstallState(t, service, req, "succeeded")
	instances, _ = store.ListGlobal(context.Background())
	if instances[0].Status != StatusRunning {
		t.Fatalf("ready backend status=%s", instances[0].Status)
	}
}

func TestInterruptedRunRecoversInstanceAndCanBeRetried(t *testing.T) {
	app := serviceApplicationAt("1")
	store := &fakeStore{global: []Instance{installedAt("old", "", "1", StatusInstalling)}}
	runs := &memoryInstallRuns{}
	_ = runs.Put(context.Background(), InstallRun{ID: "old-run", ApplicationID: app.ID, Scope: ScopeGlobal, State: "running", Log: "previous output"})
	service := New(&singleApplicationRegistry{application: app}, store, &recordingInstaller{}, nil, &countingAllocator{}, WithInstallRuns(context.Background(), runs))
	run, found, err := service.Installation(context.Background(), ScopeGlobal, "", app.ID)
	if err != nil || !found || run.State != "failed" || !strings.Contains(run.Error, "server restart") {
		t.Fatalf("recovery=%+v %v", run, err)
	}
	if _, err := service.StartInstall(context.Background(), InstallRequest{ApplicationID: app.ID, Scope: ScopeGlobal}); err != nil {
		t.Fatal(err)
	}
	awaitInstallState(t, service, InstallRequest{ApplicationID: app.ID, Scope: ScopeGlobal}, "succeeded")
	instances, _ := store.ListGlobal(context.Background())
	if len(instances) != 1 || instances[0].ID == "old" || instances[0].Status != StatusRunning {
		t.Fatalf("recovered retry=%+v", instances)
	}
}

func TestInstallLogRedactsSecretsSplitAcrossEveryByteBoundary(t *testing.T) {
	secret := "a multi-line\nsecret-value"
	encoded := base64.StdEncoding.EncodeToString([]byte(secret))
	for _, value := range []string{secret, encoded} {
		for split := 1; split < len(value); split++ {
			log := installLog{}
			log.protect(Application{Env: []EnvVar{{Key: "KEY", Secret: true}}}, map[string]string{"KEY": secret})
			log.write("before\n" + value[:split])
			if log.snapshot() != "before\n" {
				t.Fatalf("partial secret exposed at %d: %q", split, log.snapshot())
			}
			log.write(value[split:] + "\nafter")
			log.finish()
			if log.snapshot() != "before\n[redacted]\nafter" {
				t.Fatalf("redaction=%q", log.snapshot())
			}
		}
	}
	log := installLog{}
	log.write(strings.Repeat("x", installLogLimit*2) + "\nlast line")
	if len(log.snapshot()) > installLogLimit || !strings.HasSuffix(log.snapshot(), "last line") {
		t.Fatal("log tail is not bounded")
	}
}

func TestGlobalInstallStopsAtFirstFailureAndRetrySkipsCompletedProjects(t *testing.T) {
	app := portlessInfrastructureApplication()
	app.Scopes, app.GloballyInstalledInsideContainers = []Scope{ScopeGlobal}, true
	installer := &projectFailureInstaller{failProject: "p2"}
	store := &fakeStore{}
	service := New(&singleApplicationRegistry{application: app}, store, installer,
		&staticProjects{projectIDs: []string{"p1", "p2", "p3"}, container: "project"}, nil)
	req := InstallRequest{ApplicationID: app.ID, Scope: ScopeGlobal}
	if _, err := service.Install(context.Background(), req); err == nil {
		t.Fatal("connection failure reported success")
	}
	if strings.Join(installer.projects, ",") != "p1,p2" {
		t.Fatalf("continued after failure: %v", installer.projects)
	}
	installer.failProject = ""
	if _, err := service.Install(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if strings.Join(installer.projects, ",") != "p1,p2,p2,p3" {
		t.Fatalf("retry repeated completed work: %v", installer.projects)
	}
}

type projectFailureInstaller struct {
	recordingInstaller
	failProject string
	projects    []string
}

func (i *projectFailureInstaller) Install(_ context.Context, spec InstallSpec) error {
	i.projects = append(i.projects, spec.Instance.ProjectID)
	if spec.Instance.ProjectID == i.failProject {
		return errors.New("connection reset")
	}
	return nil
}
