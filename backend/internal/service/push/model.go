package push

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrDisabled            = errors.New("push notifications are not configured")
	ErrInvalidIdentity     = errors.New("push subscriptions require an authenticated user")
	ErrInvalidEndpoint     = errors.New("push subscription endpoint must be an https URL")
	ErrInvalidKeys         = errors.New("push subscription keys are malformed")
	ErrTooManySubscription = errors.New("too many push subscriptions for this user")
)

// MaxSubscriptionsPerUser bounds one user's stored devices. Browsers mint a new
// subscription whenever their old one is invalidated, so without a cap a single
// user's file would grow forever.
const MaxSubscriptionsPerUser = 20

// Subscription is one browser's push registration. Endpoint identifies it;
// P256dh and Auth are the keys every payload for it is encrypted to.
type Subscription struct {
	Endpoint   string `json:"endpoint"`
	P256dh     string `json:"p256dh"`
	Auth       string `json:"auth"`
	UserAgent  string `json:"userAgent,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	LastSentAt int64  `json:"lastSentAt,omitempty"`
}

// Kind labels why a notification was raised. The service worker uses it to
// pick an icon and whether to demand attention.
type Kind string

const (
	// KindQuestion is the agent blocking on an AskUserQuestion tool call.
	KindQuestion Kind = "question"
	// KindComplete is a finished interactive turn.
	KindComplete Kind = "complete"
	// KindError is a failed run.
	KindError Kind = "error"
	// KindScheduled is a scheduled task's run finishing, successfully or not.
	KindScheduled Kind = "scheduled"
	// KindTest is the "send me one now" button in settings.
	KindTest Kind = "test"
)

// Notification is the payload delivered to the service worker. It is
// deliberately small: push services cap the encrypted body, and the page
// re-fetches real state once the user taps through.
type Notification struct {
	Kind   Kind   `json:"kind"`
	ChatID string `json:"chatId,omitempty"`
	Title  string `json:"title"`
	Body   string `json:"body,omitempty"`
	// Tag collapses notifications: a newer one replaces the tray entry of an
	// older one sharing its tag, so a busy chat cannot spam the shade.
	Tag string `json:"tag,omitempty"`
	// Urgent asks the device to deliver promptly rather than batch with the
	// next wakeup. Reserved for the agent actually waiting on an answer.
	Urgent bool `json:"-"`
}

// Validate normalizes a subscription submitted by a browser and rejects
// anything that could not be a real Web Push registration.
func (s *Subscription) Validate() error {
	s.Endpoint = strings.TrimSpace(s.Endpoint)
	s.P256dh = strings.TrimSpace(s.P256dh)
	s.Auth = strings.TrimSpace(s.Auth)
	s.UserAgent = truncate(strings.TrimSpace(s.UserAgent), 256)

	endpoint, err := url.Parse(s.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return ErrInvalidEndpoint
	}
	if len(s.Endpoint) > 2048 {
		return ErrInvalidEndpoint
	}
	// A P-256 point is 65 bytes and the auth secret is 16, which base64url
	// encodes to 88 and 22 characters. Checking the encoded length here keeps
	// junk out of the store without decoding on every request.
	if len(s.P256dh) < 80 || len(s.P256dh) > 100 || len(s.Auth) < 16 || len(s.Auth) > 32 {
		return ErrInvalidKeys
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
