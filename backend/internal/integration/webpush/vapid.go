// Package webpush speaks the Web Push protocol (RFC 8030) with VAPID
// application-server authentication (RFC 8292) and aes128gcm payload
// encryption (RFC 8291), using only the Go standard library.
//
// The push service itself only ever sees ciphertext: payloads are encrypted to
// the key pair the browser minted for this subscription, so Google/Apple/
// Mozilla relay bytes they cannot read.
package webpush

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

// vapidTokenLifetime is how long a signed VAPID JWT stays valid. RFC 8292
// caps this at 24h; 12h leaves room for clock skew on either end.
const vapidTokenLifetime = 12 * time.Hour

var b64 = base64.RawURLEncoding

// VAPIDKey is the server's application key pair. The public half is what the
// browser receives as `applicationServerKey`; the private half signs the JWT
// that proves a push request came from this server.
type VAPIDKey struct {
	private *ecdsa.PrivateKey
	public  []byte // uncompressed P-256 point, 65 bytes
}

// GenerateVAPIDKey mints a fresh P-256 application key pair.
func GenerateVAPIDKey() (VAPIDKey, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return VAPIDKey{}, fmt.Errorf("generate vapid key: %w", err)
	}
	public, err := private.ECDH()
	if err != nil {
		return VAPIDKey{}, fmt.Errorf("derive vapid public key: %w", err)
	}
	return VAPIDKey{private: private, public: public.PublicKey().Bytes()}, nil
}

// ParseVAPIDKey rebuilds a key pair from the base64url-encoded 32-byte private
// scalar produced by VAPIDKey.PrivateKeyBase64.
func ParseVAPIDKey(privateBase64 string) (VAPIDKey, error) {
	raw, err := decodeBase64(privateBase64)
	if err != nil {
		return VAPIDKey{}, fmt.Errorf("decode vapid private key: %w", err)
	}
	if len(raw) != 32 {
		return VAPIDKey{}, fmt.Errorf("vapid private key must be 32 bytes, got %d", len(raw))
	}

	// crypto/ecdh does the range check and the base-point multiplication; the
	// resulting point feeds the ecdsa key that signs VAPID tokens.
	agreement, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return VAPIDKey{}, fmt.Errorf("invalid vapid private key: %w", err)
	}
	public := agreement.PublicKey().Bytes()
	return VAPIDKey{
		private: &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     new(big.Int).SetBytes(public[1:33]),
				Y:     new(big.Int).SetBytes(public[33:]),
			},
			D: new(big.Int).SetBytes(raw),
		},
		public: public,
	}, nil
}

// PublicKeyBase64 is the value the browser passes to pushManager.subscribe.
func (k VAPIDKey) PublicKeyBase64() string {
	return b64.EncodeToString(k.public)
}

// PrivateKeyBase64 is the persisted form of the private scalar.
func (k VAPIDKey) PrivateKeyBase64() string {
	return b64.EncodeToString(k.private.D.FillBytes(make([]byte, 32)))
}

func (k VAPIDKey) valid() bool { return k.private != nil && len(k.public) == 65 }

// authorization builds the `Authorization: vapid t=<jwt>, k=<key>` header that
// identifies this server to the push service.
func (k VAPIDKey) authorization(endpoint, subject string, now time.Time) (string, error) {
	audience, err := pushAudience(endpoint)
	if err != nil {
		return "", err
	}
	token, err := k.signJWT(audience, subject, now)
	if err != nil {
		return "", err
	}
	return "vapid t=" + token + ", k=" + k.PublicKeyBase64(), nil
}

func (k VAPIDKey) signJWT(audience, subject string, now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"aud": audience,
		"exp": now.Add(vapidTokenLifetime).Unix(),
		"sub": subject,
	})
	if err != nil {
		return "", err
	}

	signingInput := b64.EncodeToString(header) + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, k.private, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign vapid token: %w", err)
	}

	// ES256 wants the raw R||S pair, not the ASN.1 envelope ecdsa.SignASN1
	// would produce.
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signingInput + "." + b64.EncodeToString(signature), nil
}

// pushAudience is the scheme+host of the endpoint, which is what the push
// service checks the `aud` claim against.
func pushAudience(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse push endpoint: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("push endpoint must be an absolute https URL")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

// NormalizeSubject coerces the VAPID `sub` claim into the "mailto:" or
// "https://" form RFC 8292 requires. It identifies whoever operates this
// server so a push provider can reach them about abuse.
func NormalizeSubject(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("vapid subject is required")
	}
	if strings.HasPrefix(subject, "mailto:") || strings.HasPrefix(subject, "https://") {
		return strings.TrimSuffix(subject, "/"), nil
	}
	if strings.Contains(subject, "@") {
		return "mailto:" + subject, nil
	}
	return "", fmt.Errorf("vapid subject %q must be a mailto: or https:// URL", subject)
}

// publicKeyFromBytes validates an uncompressed P-256 point sent by a browser.
func publicKeyFromBytes(raw []byte) (*ecdh.PublicKey, error) {
	key, err := ecdh.P256().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription public key: %w", err)
	}
	return key, nil
}

func decodeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	// Browsers hand back unpadded base64url, but subscriptions copied through
	// other tooling sometimes arrive padded or in the standard alphabet.
	value = strings.NewReplacer("-", "+", "_", "/").Replace(value)
	value = strings.TrimRight(value, "=")
	return base64.RawStdEncoding.DecodeString(value)
}
