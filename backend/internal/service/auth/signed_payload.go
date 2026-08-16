package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// expirer is implemented by any payload type carried through signedPayload,
// so verify can enforce expiry generically without depending on any one
// payload shape (Session, a pending-login challenge, a pending-enrollment
// token, ...). Each payload type owns its own expiry field/computation.
type expirer interface {
	expired(now time.Time) bool
}

// signedPayload is a small generic HMAC sign/verify/expire primitive,
// extracted from what was SessionCodec so the same envelope format
// (b64(json) + "." + b64(hmac)) can back real sessions, pending 2FA-login
// challenges, and pending 2FA-enrollment tokens without three copies of this
// logic.
type signedPayload[T expirer] struct {
	key []byte
}

func newSignedPayload[T expirer](key []byte) signedPayload[T] {
	return signedPayload[T]{key: key}
}

func (p signedPayload[T]) sign(v T) string {
	body, _ := json.Marshal(v)
	b64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, p.key)
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

func (p signedPayload[T]) verify(raw string) (T, error) {
	var zero T
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return zero, errors.New("malformed")
	}
	mac := hmac.New(sha256.New, p.key)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		return zero, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, err
	}
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		return zero, err
	}
	if v.expired(time.Now()) {
		return zero, errors.New("expired")
	}
	return v, nil
}
