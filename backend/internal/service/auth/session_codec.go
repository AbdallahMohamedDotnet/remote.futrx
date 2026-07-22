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

const sessionDuration = 30 * 24 * time.Hour

// SessionCodec owns the signed session representation. Its methods stay
// package-private so Service remains the supported authentication facade.
type SessionCodec struct {
	key []byte
}

func newSessionCodec(key []byte) *SessionCodec {
	return &SessionCodec{key: key}
}

func (c *SessionCodec) sign(user User) string {
	now := time.Now()
	session := Session{
		Email: user.Email,
		Sub:   user.Sub,
		Iat:   now.Unix(),
		Exp:   now.Add(sessionDuration).Unix(),
	}
	body, _ := json.Marshal(session)
	b64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

func (c *SessionCodec) verify(value string) (*Session, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed")
	}
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		return nil, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, err
	}
	if time.Now().Unix() > session.Exp {
		return nil, errors.New("expired")
	}
	return &session, nil
}
