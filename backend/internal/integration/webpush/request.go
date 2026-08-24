package webpush

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// requestBuilder owns the Web Push wire representation: subscription key
// decoding, payload encryption, VAPID authorization, and protocol headers.
// Client remains responsible only for executing the request and interpreting
// the push service's response.
type requestBuilder struct {
	key     VAPIDKey
	subject string
	now     func() time.Time
}

func (b requestBuilder) publicKey() string {
	return b.key.PublicKeyBase64()
}

func (b requestBuilder) build(
	ctx context.Context,
	subscription Subscription,
	payload []byte,
	options Options,
) (*http.Request, error) {
	endpoint := strings.TrimSpace(subscription.Endpoint)
	if _, err := pushAudience(endpoint); err != nil {
		return nil, err
	}
	userAgentPublicKey, err := decodeBase64(subscription.P256dh)
	if err != nil {
		return nil, fmt.Errorf("decode subscription p256dh: %w", err)
	}
	authSecret, err := decodeBase64(subscription.Auth)
	if err != nil {
		return nil, fmt.Errorf("decode subscription auth: %w", err)
	}

	body, err := encrypt(payload, userAgentPublicKey, authSecret, nil)
	if err != nil {
		return nil, err
	}
	authorization, err := b.key.authorization(endpoint, b.subject, b.now())
	if err != nil {
		return nil, err
	}

	ttl := options.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	urgency := options.Urgency
	if urgency == "" {
		urgency = UrgencyNormal
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build push request: %w", err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	request.Header.Set("Urgency", string(urgency))
	if options.Topic != "" {
		request.Header.Set("Topic", options.Topic)
	}
	return request, nil
}
