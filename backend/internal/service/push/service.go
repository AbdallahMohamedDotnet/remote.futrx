// Package push keeps per-user Web Push registrations and fans notifications
// out to them. It knows nothing about why a notification was raised: callers
// decide the audience and the copy.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

// deliveryTimeout bounds one fan-out. Push services are third parties on the
// public internet; a wedged one must not pin a goroutine forever.
const deliveryTimeout = 30 * time.Second

type Service struct {
	repo   Repository
	sender Sender

	// wg lets tests (and a future graceful shutdown) wait for in-flight
	// deliveries that were started in the background.
	wg sync.WaitGroup
}

func New(repo Repository, sender Sender) *Service {
	if repo == nil || sender == nil {
		return &Service{}
	}
	return &Service{repo: repo, sender: sender}
}

// Enabled reports whether push is configured. It is false when the deployment
// could not build a VAPID key, and every entry point degrades to a no-op.
func (s *Service) Enabled() bool {
	return s != nil && s.repo != nil && s.sender != nil
}

// PublicKey is the applicationServerKey the browser needs to subscribe.
func (s *Service) PublicKey() string {
	if !s.Enabled() {
		return ""
	}
	return s.sender.PublicKey()
}

// Subscribe registers (or refreshes) one device for a user.
func (s *Service) Subscribe(ctx context.Context, email string, subscription Subscription) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	email = NormalizeEmail(email)
	if email == "" {
		return ErrInvalidIdentity
	}
	if err := subscription.Validate(); err != nil {
		return err
	}

	existing, err := s.repo.List(ctx, email)
	if err != nil {
		return err
	}
	// Re-subscribing the same endpoint is a refresh, not a new device, so it
	// never counts against the cap.
	known := false
	for _, candidate := range existing {
		if candidate.Endpoint == subscription.Endpoint {
			known = true
			subscription.CreatedAt = candidate.CreatedAt
			break
		}
	}
	if !known && len(existing) >= MaxSubscriptionsPerUser {
		return ErrTooManySubscription
	}
	if subscription.CreatedAt == 0 {
		subscription.CreatedAt = time.Now().UnixMilli()
	}
	return s.repo.Save(ctx, email, subscription)
}

// Unsubscribe drops one device. Removing an endpoint that was never stored is
// not an error: the browser may be retrying a cleanup.
func (s *Service) Unsubscribe(ctx context.Context, email, endpoint string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	email = NormalizeEmail(email)
	if email == "" {
		return ErrInvalidIdentity
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ErrInvalidEndpoint
	}
	return s.repo.Delete(ctx, email, endpoint)
}

// HasSubscriptions reports whether a user has any device registered, so the UI
// can show whether this account is reachable at all.
func (s *Service) HasSubscriptions(ctx context.Context, email string) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	subscriptions, err := s.repo.List(ctx, NormalizeEmail(email))
	if err != nil {
		return false, err
	}
	return len(subscriptions) > 0, nil
}

// Notify delivers to every device of every recipient, pruning subscriptions
// the push service reports as retired. It blocks; use NotifyAsync from
// latency-sensitive paths.
func (s *Service) Notify(ctx context.Context, recipients []string, notification Notification) {
	if !s.Enabled() || len(recipients) == 0 {
		return
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		log.Printf("push: encode notification: %v", err)
		return
	}

	for _, email := range dedupeEmails(recipients) {
		subscriptions, err := s.repo.List(ctx, email)
		if err != nil {
			log.Printf("push: list subscriptions: %v", err)
			continue
		}
		for _, subscription := range subscriptions {
			s.deliver(ctx, email, subscription, payload, notification.Urgent)
		}
	}
}

// NotifyAsync runs Notify on its own goroutine with an independent deadline.
// Chat events are appended on the streaming hot path, and a slow push service
// must never hold that up.
func (s *Service) NotifyAsync(recipients []string, notification Notification) {
	if !s.Enabled() || len(recipients) == 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
		defer cancel()
		s.Notify(ctx, recipients, notification)
	}()
}

// Wait blocks until background deliveries finish. Used by tests.
func (s *Service) Wait() {
	if s == nil {
		return
	}
	s.wg.Wait()
}

func (s *Service) deliver(
	ctx context.Context,
	email string,
	subscription Subscription,
	payload []byte,
	urgent bool,
) {
	err := s.sender.Send(ctx, subscription, payload, urgent)
	switch {
	case err == nil:
		subscription.LastSentAt = time.Now().UnixMilli()
		if saveErr := s.repo.Save(ctx, email, subscription); saveErr != nil {
			log.Printf("push: record delivery: %v", saveErr)
		}
	case errors.Is(err, ErrGone):
		// The browser dropped this registration. Forget it so the next
		// fan-out is not slowed down by a dead endpoint.
		if deleteErr := s.repo.Delete(ctx, email, subscription.Endpoint); deleteErr != nil {
			log.Printf("push: prune retired subscription: %v", deleteErr)
		}
	default:
		log.Printf("push: deliver to %s: %v", endpointHost(subscription.Endpoint), err)
	}
}

// NormalizeEmail is the identity form used as the subscription store key and
// for matching against project access lists.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func dedupeEmails(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	out := make([]string, 0, len(emails))
	for _, email := range emails {
		email = NormalizeEmail(email)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}

// endpointHost keeps push-service hostnames in logs without the subscription
// id, which is a bearer capability to notify that device.
func endpointHost(endpoint string) string {
	trimmed := strings.TrimPrefix(endpoint, "https://")
	if index := strings.IndexByte(trimmed, '/'); index >= 0 {
		return trimmed[:index]
	}
	return trimmed
}
