package projects

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Workspace dir for a project — bind-mounted into the future container at /workspace.
// Kept under /var/lib so it doesn't clutter $HOME and survives an `lxc delete`.
const WorkspaceRoot = "/var/lib/remote/projects"

// Store manages project metadata. Mirrors ChatStore's filesystem-backed style.
type Store struct {
	root          string
	workspaceRoot string
	manager       *Manager
	mu            sync.Mutex
	locks         map[string]*sync.Mutex
	indexMu       sync.RWMutex
	bySlug        map[string]string // slug -> id (used for slug-collision lookups)
}

func NewStore(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "projects")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create projects dir: %w", err)
	}
	if err := os.MkdirAll(WorkspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}
	s := &Store{
		root:          dir,
		workspaceRoot: WorkspaceRoot,
		manager:       NewManager(),
		locks:         map[string]*sync.Mutex{},
		bySlug:        map[string]string{},
	}
	if err := s.loadSlugIndex(); err != nil {
		return nil, err
	}
	// Best-effort reconcile of meta statuses with live container reality.
	if err := s.Reconcile(context.Background()); err != nil {
		log.Printf("projects: reconcile warning: %v", err)
	}
	return s, nil
}

// Manager exposes the LXC wrapper to callers that need it (e.g. claude.runner
// in task #9 for `lxc exec`).
func (s *Store) Manager() *Manager { return s.manager }

func (s *Store) projectDir(id string) string {
	return filepath.Join(s.root, id)
}

func (s *Store) WorkspaceDir(slug string) string {
	return filepath.Join(s.workspaceRoot, slug, "workspace")
}

func (s *Store) lock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.locks[id]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.locks[id] = m
	return m
}

func newProjectID() string {
	var b [6]byte
	_, _ = crand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// --- on-disk read/write --------------------------------------------------------

func (s *Store) readMeta(id string) (ProjectMeta, error) {
	data, err := os.ReadFile(filepath.Join(s.projectDir(id), "meta.json"))
	if err != nil {
		return ProjectMeta{}, err
	}
	var m ProjectMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return ProjectMeta{}, fmt.Errorf("parse meta for %s: %w", id, err)
	}
	return m, nil
}

func (s *Store) writeMeta(m ProjectMeta) error {
	dir := s.projectDir(m.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".meta.json.tmp")
	final := filepath.Join(dir, "meta.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (s *Store) loadSlugIndex() error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := s.readMeta(e.Name())
		if err != nil {
			continue
		}
		s.bySlug[m.Slug] = m.ID
	}
	return nil
}

// --- public API ----------------------------------------------------------------

// resolveSlug picks an unused slug. If `base` is free, returns it. Otherwise
// appends -2, -3, … until something works (or we run out of attempts).
func (s *Store) resolveSlug(base string) (string, error) {
	s.indexMu.RLock()
	_, taken := s.bySlug[base]
	s.indexMu.RUnlock()
	if !taken {
		return base, nil
	}
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if len(cand) > maxSlugLen {
			cand = fmt.Sprintf("%s-%d", base[:maxSlugLen-len(fmt.Sprintf("-%d", i))], i)
		}
		s.indexMu.RLock()
		_, taken := s.bySlug[cand]
		s.indexMu.RUnlock()
		if !taken {
			return cand, nil
		}
	}
	return "", errors.New("could not find an available slug")
}

type CreateInput struct {
	Name string
}

// Create writes the meta + workspace, then spawns the LXC container. If the
// container launch fails, the project lands in StatusError with the error
// message — the meta still exists so the user can retry (POST /start) or
// delete it. Returns the *latest* meta after container provisioning.
func (s *Store) Create(ctx context.Context, in CreateInput) (ProjectMeta, error) {
	name := in.Name
	if name == "" {
		return ProjectMeta{}, errors.New("name is required")
	}
	base := Slugify(name)
	slug, err := s.resolveSlug(base)
	if err != nil {
		return ProjectMeta{}, err
	}
	id := newProjectID()
	now := time.Now().UnixMilli()
	ws := s.WorkspaceDir(slug)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return ProjectMeta{}, fmt.Errorf("create workspace: %w", err)
	}
	m := ProjectMeta{
		ID:            id,
		Name:          name,
		Slug:          slug,
		Cwd:           ws,
		ContainerName: "proj-" + slug,
		Status:        StatusProvisioning,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.writeMeta(m); err != nil {
		return ProjectMeta{}, err
	}
	s.indexMu.Lock()
	s.bySlug[slug] = id
	s.indexMu.Unlock()

	// Launch the container synchronously. ~5–10s for unprivileged ubuntu.
	// Could be async (return immediately, poll for status) — punt for v1.
	if err := s.manager.Launch(ctx, m); err != nil {
		log.Printf("projects: launch %s failed: %v", m.ContainerName, err)
		return s.SetStatus(id, StatusError, err.Error())
	}
	return s.SetStatus(id, StatusRunning, "")
}

func (s *Store) Get(id string) (ProjectMeta, error) {
	return s.readMeta(id)
}

func (s *Store) List() ([]ProjectMeta, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectMeta, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := s.readMeta(e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

type UpdateInput struct {
	Name *string
}

func (s *Store) Update(id string, in UpdateInput) (ProjectMeta, error) {
	lk := s.lock(id)
	lk.Lock()
	defer lk.Unlock()
	m, err := s.readMeta(id)
	if err != nil {
		return ProjectMeta{}, err
	}
	if in.Name != nil && *in.Name != "" {
		m.Name = *in.Name
	}
	m.UpdatedAt = time.Now().UnixMilli()
	if err := s.writeMeta(m); err != nil {
		return ProjectMeta{}, err
	}
	return m, nil
}

// SetStatus is a focused update used by the future container provisioner.
func (s *Store) SetStatus(id string, status ProjectStatus, errMsg string) (ProjectMeta, error) {
	lk := s.lock(id)
	lk.Lock()
	defer lk.Unlock()
	m, err := s.readMeta(id)
	if err != nil {
		return ProjectMeta{}, err
	}
	m.Status = status
	m.ErrorMsg = errMsg
	m.UpdatedAt = time.Now().UnixMilli()
	if err := s.writeMeta(m); err != nil {
		return ProjectMeta{}, err
	}
	return m, nil
}

// Delete tears down the container first (force-stop + lxc delete), then
// removes meta + workspace. Safe to call repeatedly.
func (s *Store) Delete(ctx context.Context, id string) error {
	lk := s.lock(id)
	lk.Lock()
	defer lk.Unlock()
	m, err := s.readMeta(id)
	if err != nil {
		return err
	}
	// Container teardown first — failures here shouldn't block meta cleanup,
	// but we surface a useful error.
	if m.ContainerName != "" {
		if err := s.manager.Delete(ctx, m.ContainerName); err != nil {
			log.Printf("projects: delete container %s: %v", m.ContainerName, err)
		}
	}
	if err := os.RemoveAll(s.projectDir(id)); err != nil {
		return fmt.Errorf("remove project meta dir: %w", err)
	}
	if m.Cwd != "" {
		// Destructive by design (matches chat delete). Add a "keep workspace"
		// option later if it bites.
		_ = os.RemoveAll(filepath.Dir(m.Cwd)) // /var/lib/remote/projects/{slug}/
	}
	s.indexMu.Lock()
	delete(s.bySlug, m.Slug)
	s.indexMu.Unlock()
	return nil
}

// Start brings up a stopped container. No-op if already running.
func (s *Store) Start(ctx context.Context, id string) (ProjectMeta, error) {
	m, err := s.Get(id)
	if err != nil {
		return ProjectMeta{}, err
	}
	if err := s.manager.Start(ctx, m.ContainerName); err != nil {
		return s.SetStatus(id, StatusError, err.Error())
	}
	return s.SetStatus(id, StatusRunning, "")
}

// Stop shuts down a running container, preserving state.
func (s *Store) Stop(ctx context.Context, id string) (ProjectMeta, error) {
	m, err := s.Get(id)
	if err != nil {
		return ProjectMeta{}, err
	}
	if err := s.manager.Stop(ctx, m.ContainerName); err != nil {
		return s.SetStatus(id, StatusError, err.Error())
	}
	return s.SetStatus(id, StatusStopped, "")
}

// Reconcile syncs meta.Status against live LXC state. Called at startup so
// stale "provisioning" or "running" entries from a previous run get updated
// to reflect what's actually on the box.
func (s *Store) Reconcile(ctx context.Context) error {
	if !s.manager.Available() {
		return nil // LXD not installed; leave statuses alone
	}
	metas, err := s.List()
	if err != nil {
		return err
	}
	for _, m := range metas {
		state, err := s.manager.State(ctx, m.ContainerName)
		if err != nil {
			continue
		}
		var want ProjectStatus
		switch state {
		case StateRunning:
			want = StatusRunning
		case StateStopped:
			want = StatusStopped
		case StateMissing:
			want = StatusMissing
		default:
			want = StatusUnknown
		}
		if want != m.Status {
			if _, err := s.SetStatus(m.ID, want, ""); err != nil {
				log.Printf("projects: reconcile %s: %v", m.ID, err)
			}
		}
	}
	return nil
}
