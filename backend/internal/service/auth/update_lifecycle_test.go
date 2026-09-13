package auth

import (
	"context"
	"sync"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/lifecycle/publishers"
)

type blockingTwoFactorStore struct {
	mu        sync.Mutex
	available bool
	started   chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (s *blockingTwoFactorStore) Get(context.Context, string) (*TwoFactorRecord, error) {
	s.mu.Lock()
	available := s.available
	s.mu.Unlock()
	s.once.Do(func() { close(s.started) })
	<-s.release
	if !available {
		return nil, nil
	}
	return &TwoFactorRecord{Secret: []byte("persisted-secret")}, nil
}

func (*blockingTwoFactorStore) Save(context.Context, string, TwoFactorRecord) error { return nil }
func (*blockingTwoFactorStore) Delete(context.Context, string) error                { return nil }

func TestUpdateLifecycleReloadsTwoFactorEnrollmentFromDisk(t *testing.T) {
	tests := []struct {
		name   string
		notify func(*publishers.Core)
	}{
		{
			name: "started",
			notify: func(core *publishers.Core) {
				core.PublishUpdateStarted(context.Background(), "0.4.0", "application", "admin@example.com")
			},
		},
		{
			name: "succeeded",
			notify: func(core *publishers.Core) {
				core.PublishUpdateSucceeded(context.Background(), "0.4.0", "application", "admin@example.com")
			},
		},
		{
			name: "failed",
			notify: func(core *publishers.Core) {
				core.PublishUpdateFailed(context.Background(), "0.4.0", "application", "admin@example.com")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newAuthTestTwoFactorStore()
			authenticator := newTwoFactorAuthenticator(
				store,
				"remote.futrx",
				[]byte("test-key"),
				authTestOptions().EnrollmentTTL,
				authTestOptions().RecoveryCodeCount,
			)
			service := &Service{twoFactor: authenticator}
			core := publishers.NewCore()
			core.Subscribe(service)
			email := "user@example.com"

			// Cache the transient absence first, then simulate the durable record
			// becoming visible while an update is settling.
			if authenticator.Enabled(context.Background(), email) {
				t.Fatal("2FA unexpectedly enabled before the record appeared")
			}
			store.records[email] = TwoFactorRecord{Secret: []byte("persisted-secret")}
			if authenticator.Enabled(context.Background(), email) {
				t.Fatal("cached absence did not reproduce the stale view")
			}

			test.notify(core)

			if !authenticator.Enabled(context.Background(), email) {
				t.Fatal("lifecycle callback did not reload the persisted enrollment")
			}
		})
	}
}

func TestUpdateLifecycleDoesNotLetInflightLoadRestoreStaleCache(t *testing.T) {
	store := &blockingTwoFactorStore{started: make(chan struct{}), release: make(chan struct{})}
	options := authTestOptions()
	authenticator := newTwoFactorAuthenticator(
		store,
		"remote.futrx",
		[]byte("test-key"),
		options.EnrollmentTTL,
		options.RecoveryCodeCount,
	)
	service := &Service{twoFactor: authenticator}
	core := publishers.NewCore()
	core.Subscribe(service)
	email := "user@example.com"

	firstResult := make(chan bool, 1)
	go func() {
		firstResult <- authenticator.Enabled(context.Background(), email)
	}()
	<-store.started

	core.PublishUpdateSucceeded(context.Background(), "0.4.0", "application", "admin@example.com")
	store.mu.Lock()
	store.available = true
	store.mu.Unlock()
	close(store.release)

	if <-firstResult {
		t.Fatal("the already in-flight read unexpectedly saw the later record")
	}
	if !authenticator.Enabled(context.Background(), email) {
		t.Fatal("the stale in-flight read was cached after lifecycle invalidation")
	}
}
