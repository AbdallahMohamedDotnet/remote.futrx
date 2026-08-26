package auth

import "time"

const sessionDuration = 30 * 24 * time.Hour

func (s Session) expired(now time.Time) bool {
	return now.Unix() > s.Exp
}

// SessionCodec owns the signed session representation. Its methods stay
// package-private so Service remains the supported authentication facade.
type SessionCodec struct {
	payload signedPayload[Session]
}

func newSessionCodec(key []byte) *SessionCodec {
	return &SessionCodec{payload: newSignedPayload[Session](key)}
}

func (c *SessionCodec) sign(user User, sid string) string {
	now := time.Now()
	session := Session{
		Email: user.Email,
		Sub:   user.Sub,
		Iat:   now.Unix(),
		Exp:   now.Add(sessionDuration).Unix(),
		SID:   sid,
	}
	return c.payload.sign(session)
}

func (c *SessionCodec) verify(value string) (*Session, error) {
	session, err := c.payload.verify(value)
	if err != nil {
		return nil, err
	}
	return &session, nil
}
