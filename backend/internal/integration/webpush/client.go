package webpush

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	key     VAPIDKey
	subject string
	http    *http.Client
	now     func() time.Time
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
		key:     key,
		subject: subject,
		http:    &http.Client{Timeout: 15 * time.Second},
		now:     time.Now,
	}, nil
}

// PublicKey is the applicationServerKey browsers need to subscribe.
func (c *Client) PublicKey() string { return c.key.PublicKeyBase64() }

// Send encrypts payload for one subscription and hands it to its push service.
func (c *Client) Send(ctx context.Context, sub Subscription, payload []byte, opts Options) error {
	endpoint := strings.TrimSpace(sub.Endpoint)
	if _, err := pushAudience(endpoint); err != nil {
		return err
	}
	uaPublic, err := decodeBase64(sub.P256dh)
	if err != nil {
		return fmt.Errorf("decode subscription p256dh: %w", err)
	}
	authSecret, err := decodeBase64(sub.Auth)
	if err != nil {
		return fmt.Errorf("decode subscription auth: %w", err)
	}

	body, err := encrypt(payload, uaPublic, authSecret, nil)
	if err != nil {
		return err
	}
	authorization, err := c.key.authorization(endpoint, c.subject, c.now())
	if err != nil {
		return err
	}

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	urgency := opts.Urgency
	if urgency == "" {
		urgency = UrgencyNormal
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	request.Header.Set("Urgency", string(urgency))
	if opts.Topic != "" {
		request.Header.Set("Topic", opts.Topic)
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
