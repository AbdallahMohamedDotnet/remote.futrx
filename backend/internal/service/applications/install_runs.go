package applications

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrInstallInProgress = errors.New("applications: an installation is already running")

// InstallRun is the latest attempt for an application in a particular scope.
// It never contains install inputs or resolved credentials.
type InstallRun struct {
	ID            string          `json:"id"`
	ApplicationID string          `json:"applicationId"`
	Scope         Scope           `json:"scope"`
	ProjectID     string          `json:"projectId,omitempty"`
	State         string          `json:"state"`
	StartedAt     int64           `json:"startedAt"`
	FinishedAt    int64           `json:"finishedAt,omitempty"`
	Error         string          `json:"error,omitempty"`
	Log           string          `json:"log,omitempty"`
	LogUpdatedAt  int64           `json:"logUpdatedAt,omitempty"`
	Progress      InstallProgress `json:"progress"`
}

type InstallRunStore interface {
	Get(context.Context, Scope, string, string) (InstallRun, bool, error)
	Put(context.Context, InstallRun) error
}

// WithInstallRuns binds observed installs to server shutdown and durable state.
// The legacy synchronous Install API remains available to application tooling.
func WithInstallRuns(ctx context.Context, store InstallRunStore) Option {
	return func(s *Service) {
		s.installRuns.ctx, s.installRuns.store = ctx, store
	}
}

type installRunKey struct {
	scope        Scope
	project, app string
}

type installRuns struct {
	mu    sync.Mutex
	ctx   context.Context
	store InstallRunStore
	runs  map[installRunKey]*observedInstall
}

type observedInstall struct {
	mu       sync.Mutex
	run      InstallRun
	log      installLog
	store    InstallRunStore
	lastSave time.Time
}

func (s *Service) StartInstall(ctx context.Context, req InstallRequest) (InstallRun, error) {
	application, ok := s.registry.Get(req.ApplicationID)
	if !ok {
		return InstallRun{}, ErrUnknownApplication
	}
	if !req.Scope.Valid() || !application.SupportsScope(req.Scope) {
		return InstallRun{}, ErrScope
	}
	if req.Scope == ScopeProject && req.ProjectID == "" {
		return InstallRun{}, ErrProjectneeded
	}
	if req.ExternalPort < 0 || req.ExternalPort > 65535 {
		return InstallRun{}, ErrPortRange
	}
	if _, err := resolveEnv(application, req.Env); err != nil {
		return InstallRun{}, err
	}
	if err := ctx.Err(); err != nil {
		return InstallRun{}, err
	}
	// Do not let a caller mutate values after the worker takes ownership.
	env := make(map[string]string, len(req.Env))
	for key, value := range req.Env {
		env[key] = value
	}
	req.Env = env
	key := installRunKey{req.Scope, req.ProjectID, req.ApplicationID}
	m := &s.installRuns
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs == nil {
		m.runs = make(map[installRunKey]*observedInstall)
	}
	if previous := m.runs[key]; previous != nil && previous.snapshot().State == "running" {
		return previous.snapshot(), ErrInstallInProgress
	}
	if m.runs[key] == nil && m.store != nil {
		previous, found, err := m.store.Get(ctx, req.Scope, req.ProjectID, req.ApplicationID)
		if err != nil {
			return InstallRun{}, err
		}
		if found && previous.State == "running" {
			if err := s.recoverInterruptedInstall(ctx, req.Scope, req.ProjectID, req.ApplicationID); err != nil {
				return InstallRun{}, err
			}
		}
	}
	if !(req.Scope == ScopeGlobal && application.GloballyInstalledInsideContainers) {
		existing, found, err := s.instanceOfApplication(ctx, req.Scope, req.ProjectID, req.ApplicationID)
		if err != nil {
			return InstallRun{}, err
		}
		if found && existing.Status == StatusInstalling {
			return InstallRun{}, ErrInstallInProgress
		}
		if found && existing.Status != StatusError {
			return InstallRun{}, ErrAlreadyInstalled
		}
	}
	workerCtx := m.ctx
	if workerCtx == nil {
		workerCtx = context.Background()
	}
	if err := workerCtx.Err(); err != nil {
		return InstallRun{}, err
	}
	run := &observedInstall{
		run: InstallRun{ID: newInstanceID(), ApplicationID: req.ApplicationID, Scope: req.Scope,
			ProjectID: req.ProjectID, State: "running", StartedAt: s.now(),
			Progress: InstallProgress{Phase: "preparing", Message: "Preparing installation", UpdatedAt: s.now()}},
		store: m.store,
	}
	run.log.protect(application, req.Env)
	if err := run.save(ctx); err != nil {
		return InstallRun{}, err
	}
	m.runs[key] = run
	initial := run.snapshot()
	go func() {
		_, err := s.Install(observeInstall(workerCtx, run), req)
		run.finish(err)
	}()
	return initial, nil
}

// Installation returns scope-bound state. A persisted running attempt without
// a worker was interrupted by a server restart and becomes a retryable failure.
func (s *Service) Installation(ctx context.Context, scope Scope, project, app string) (InstallRun, bool, error) {
	m := &s.installRuns
	m.mu.Lock()
	defer m.mu.Unlock()
	key := installRunKey{scope, project, app}
	if run := m.runs[key]; run != nil {
		return run.snapshot(), true, nil
	}
	if m.store == nil {
		return InstallRun{}, false, nil
	}
	run, found, err := m.store.Get(ctx, scope, project, app)
	if err != nil || !found {
		return run, found, err
	}
	if run.State == "running" {
		run.State, run.Error, run.FinishedAt = "failed", "Installation interrupted by a server restart. Retry to try again.", s.now()
		// A crash may also have left a persisted instance marked installing.
		if err := s.recoverInterruptedInstall(ctx, scope, project, app); err != nil {
			return InstallRun{}, false, err
		}
		if err := m.store.Put(ctx, run); err != nil {
			return InstallRun{}, false, err
		}
	}
	return run, true, nil
}

func (s *Service) Installations(ctx context.Context, scope Scope, project string) ([]InstallRun, error) {
	runs := []InstallRun{}
	for _, application := range s.registry.List() {
		run, found, err := s.Installation(ctx, scope, project, application.ID)
		if err != nil {
			return nil, err
		}
		if found {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *Service) recoverInterruptedInstall(ctx context.Context, scope Scope, project, app string) error {
	instances, err := s.store.ListAll(ctx)
	if err != nil {
		return err
	}
	application, _ := s.registry.Get(app)
	for _, inst := range instances {
		matches := inst.Scope == scope && inst.ProjectID == project
		if scope == ScopeGlobal && application.GloballyInstalledInsideContainers {
			matches = inst.Scope == ScopeProject
		}
		if matches && inst.ApplicationID == app && inst.Status == StatusInstalling {
			unlock, acquired := s.instanceLocks.tryLock(inst.ID)
			if !acquired {
				continue
			}
			err := s.saveStatus(ctx, &inst, StatusError, "Installation interrupted by a server restart. Retry to try again.")
			unlock()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *observedInstall) snapshot() InstallRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *observedInstall) snapshotLocked() InstallRun {
	run := r.run
	run.Log = r.log.snapshot()
	return run
}

func (r *observedInstall) save(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	return r.store.Put(ctx, r.snapshotLocked())
}

func (r *observedInstall) progress(progress InstallProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.run.State != "running" {
		return
	}
	progress.UpdatedAt = time.Now().Unix()
	if progress.Total == 0 {
		progress.CurrentItem, progress.Completed, progress.Total = r.run.Progress.CurrentItem, r.run.Progress.Completed, r.run.Progress.Total
	}
	r.run.Progress = progress
	r.log.finish()
	r.log.write("\n" + progress.Message + "\n")
	r.run.LogUpdatedAt = progress.UpdatedAt
	_ = r.save(context.Background())
}

func (r *observedInstall) protect(application Application, env map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log.protect(application, env)
}

func (r *observedInstall) redact(text string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.log.redact(text)
}

func (r *observedInstall) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.run.State != "running" {
		return len(data), nil
	}
	r.log.write(string(data))
	r.run.LogUpdatedAt = time.Now().Unix()
	if time.Since(r.lastSave) >= time.Second {
		_ = r.save(context.Background())
		r.lastSave = time.Now()
	}
	return len(data), nil
}

func (r *observedInstall) finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.State, r.run.FinishedAt = "succeeded", time.Now().Unix()
	r.log.finish()
	if err != nil {
		r.run.State = "failed"
		r.run.Error = r.log.redact(err.Error())
		r.log.write("\nInstallation stopped: " + err.Error() + "\n")
	} else {
		r.run.Progress.Phase, r.run.Progress.Message = "complete", "Application installed"
		r.log.write("\nApplication installed\n")
	}
	r.log.finish()
	r.run.Progress.UpdatedAt, r.run.LogUpdatedAt = r.run.FinishedAt, r.run.FinishedAt
	if saveErr := r.save(context.Background()); saveErr != nil {
		r.run.State, r.run.Error = "failed", fmt.Sprintf("Could not save installation result: %v", saveErr)
	}
}
