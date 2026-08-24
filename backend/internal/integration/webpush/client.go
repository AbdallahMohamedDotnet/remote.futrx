package webpush

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrSubscriptionGone reports that the push service has permanently retired an
// endpoint. The caller should drop the stored subscription rather than retry.
var ErrSubscriptionGone = errors.New("push subscription is gone")

// Subscription is the browser-minted PushSubscription: where to deliver, and
// the keys the payload is encrypted to.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Urgency lets a device defer low-value messages while on battery saver.
type Urgency string

const (
	UrgencyNormal Urgency = "normal"
	UrgencyHigh   Urgency = "high"
)

// Options tune a single delivery.
type Options struct {
	// TTL is how long the push service may hold an undelivered message.
	TTL time.Duration
	// Urgency defaults to normal.
	Urgency Urgency
	// Topic collapses undelivered messages: a newer message with the same
	// topic replaces an older one still queued at the push service.
	Topic string
}

// Client sends encrypted notifications to push services on behalf of one
// application server identity.
type Client struct {
	requests requestBuilder
	http     *http.Client
}

// NewClient binds a VAPID key pair to a contact subject (a mailto: or https://
// URL identifying whoever runs this server).
func NewClient(key VAPIDKey, subject string) (*Client, error) {
	if !key.valid() {
		return nil, errors.New("vapid key is not initialized")
	}
	subject, err := NormalizeSubject(subject)
	if err != nil {
		return nil, err
	}
	return &Client{
		requests: requestBuilder{key: key, subject: subject, now: time.Now},
		http:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// PublicKey is the applicationServerKey browsers need to subscribe.
func (c *Client) PublicKey() string { return c.requests.publicKey() }

// Send encrypts payload for one subscription and hands it to its push service.
func (c *Client) Send(ctx context.Context, sub Subscription, payload []byte, opts Options) error {
	request, err := c.requests.build(ctx, sub, payload, opts)
	if err != nil {
		return err
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer response.Body.Close()
	// Drain enough to let the connection be reused, and to quote the failure.
	detail, _ := io.ReadAll(io.LimitReader(response.Body, 512))

	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		return nil
	case response.StatusCode == http.StatusNotFound, response.StatusCode == http.StatusGone:
		return ErrSubscriptionGone
	default:
		return fmt.Errorf(
			"push service returned %s: %s",
			response.Status,
			strings.TrimSpace(string(detail)),
		)
	}
}
