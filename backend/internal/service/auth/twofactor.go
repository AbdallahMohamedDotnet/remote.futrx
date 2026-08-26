package auth

import (
	"context"
	"encoding/base32"
	"errors"
	"sync"
	"time"
)

var (
	ErrTwoFactorNotEnabled     = errors.New("two-factor authentication is not enabled")
	ErrTwoFactorAlreadyEnabled = errors.New("two-factor authentication is already enabled")
	ErrInvalidTwoFactorCode    = errors.New("invalid two-factor code")
	ErrInvalidEnrollmentToken  = errors.New("invalid or expired enrollment token")
)

const enrollmentTTL = 10 * time.Minute

// pendingEnrollment is the signed, stateless payload carried by an
// enrollment token between BeginEnrollment and ConfirmEnrollment - nothing
// is persisted server-side until the user proves possession of the
// authenticator app.
type pendingEnrollment struct {
	Email  string `json:"email"`
	Secret []byte `json:"secret"`
	Exp    int64  `json:"exp"`
}

func (p pendingEnrollment) expired(now time.Time) bool {
	return now.Unix() > p.Exp
}

const recoveryCodeCount = 10

// TwoFactorAuthenticator owns TOTP enrollment, verification, and
// recovery-code policy for one account at a time. Mutex + in-memory cache
// per the LocalAdminAuthenticator shape: since accounts (unlike the single
// local admin) are numerous and unknown ahead of time, each account's
// record - including a confirmed "not enrolled" (nil) result - is cached
// lazily on first access, so accounts that never enroll pay at most one
// file read per process lifetime, not one per request.
type TwoFactorAuthenticator struct {
	store  TwoFactorStore
	issuer string
	codec  signedPayload[pendingEnrollment]

	mu    sync.RWMutex
	cache map[string]*TwoFactorRecord
}

func newTwoFactorAuthenticator(store TwoFactorStore, issuer string, key []byte) *TwoFactorAuthenticator {
	return &TwoFactorAuthenticator{
		store:  store,
		issuer: issuer,
		codec:  newSignedPayload[pendingEnrollment](key),
		cache:  map[string]*TwoFactorRecord{},
	}
}

func (a *TwoFactorAuthenticator) load(ctx context.Context, email string) (*TwoFactorRecord, error) {
	email = normalizeEmail(email)
	a.mu.RLock()
	if record, ok := a.cache[email]; ok {
		a.mu.RUnlock()
		return record, nil
	}
	a.mu.RUnlock()

	record, err := a.store.Get(ctx, email)
	if err != nil {
		return nil, err
	}
	a.setCache(email, record)
	return record, nil
}

func (a *TwoFactorAuthenticator) setCache(email string, record *TwoFactorRecord) {
	a.mu.Lock()
	a.cache[normalizeEmail(email)] = record
	a.mu.Unlock()
}

// Enabled reports whether email has completed TOTP enrollment.
func (a *TwoFactorAuthenticator) Enabled(ctx context.Context, email string) bool {
	record, err := a.load(ctx, email)
	if err != nil {
		return false
	}
	return record != nil
}

// BeginEnrollment generates a fresh secret and returns a signed enrollment
// token (nothing persisted yet) along with the base32 secret and otpauth URI
// for QR/manual entry.
func (a *TwoFactorAuthenticator) BeginEnrollment(ctx context.Context, email string) (enrollmentToken, secretBase32, otpauthURL string, err error) {
	email = normalizeEmail(email)
	if a.Enabled(ctx, email) {
		return "", "", "", ErrTwoFactorAlreadyEnabled
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return "", "", "", err
	}
	token := a.codec.sign(pendingEnrollment{
		Email:  email,
		Secret: secret,
		Exp:    time.Now().Add(enrollmentTTL).Unix(),
	})
	secretBase32 = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	return token, secretBase32, TOTPProvisioningURI(a.issuer, email, secret), nil
}

// ConfirmEnrollment verifies the first code from the user's authenticator
// app, persists the TwoFactorRecord, and returns a fresh set of recovery
// codes (shown to the user exactly once).
func (a *TwoFactorAuthenticator) ConfirmEnrollment(ctx context.Context, enrollmentToken, code string) (recoveryCodes []string, email string, err error) {
	pending, verifyErr := a.codec.verify(enrollmentToken)
	if verifyErr != nil {
		return nil, "", ErrInvalidEnrollmentToken
	}
	if !VerifyTOTPCode(pending.Secret, code, time.Now()) {
		return nil, "", ErrInvalidTwoFactorCode
	}
	codes, err := GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, "", err
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = HashRecoveryCode(c)
	}
	record := TwoFactorRecord{
		Secret:             pending.Secret,
		RecoveryCodeHashes: hashes,
		EnabledAt:          time.Now().Unix(),
	}
	if err := a.store.Save(ctx, pending.Email, record); err != nil {
		return nil, "", err
	}
	a.setCache(pending.Email, &record)
	return codes, pending.Email, nil
}

// VerifyChallenge checks code against email's TOTP secret, falling back to
// recovery-code consumption. usedRecoveryCode distinguishes the two so the
// caller can record the precise SignInMethod.
func (a *TwoFactorAuthenticator) VerifyChallenge(ctx context.Context, email, code string) (usedRecoveryCode bool, err error) {
	email = normalizeEmail(email)
	record, err := a.load(ctx, email)
	if err != nil {
		return false, err
	}
	if record == nil {
		return false, ErrTwoFactorNotEnabled
	}
	if VerifyTOTPCode(record.Secret, code, time.Now()) {
		return false, nil
	}
	remaining, ok := ConsumeRecoveryCode(record.RecoveryCodeHashes, code)
	if !ok {
		return false, ErrInvalidTwoFactorCode
	}
	updated := *record
	updated.RecoveryCodeHashes = remaining
	if err := a.store.Save(ctx, email, updated); err != nil {
		return false, err
	}
	a.setCache(email, &updated)
	return true, nil
}

// Disable requires proof of possession (a current TOTP code or an unused
// recovery code) before removing the account's TwoFactorRecord entirely.
func (a *TwoFactorAuthenticator) Disable(ctx context.Context, email, code string) error {
	email = normalizeEmail(email)
	record, err := a.load(ctx, email)
	if err != nil {
		return err
	}
	if record == nil {
		return ErrTwoFactorNotEnabled
	}
	if !VerifyTOTPCode(record.Secret, code, time.Now()) {
		if _, ok := ConsumeRecoveryCode(record.RecoveryCodeHashes, code); !ok {
			return ErrInvalidTwoFactorCode
		}
	}
	if err := a.store.Delete(ctx, email); err != nil {
		return err
	}
	a.setCache(email, nil)
	return nil
}

// RegenerateRecoveryCodes replaces email's recovery codes with a fresh set,
// after verifying a current TOTP code.
func (a *TwoFactorAuthenticator) RegenerateRecoveryCodes(ctx context.Context, email, code string) ([]string, error) {
	email = normalizeEmail(email)
	record, err := a.load(ctx, email)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrTwoFactorNotEnabled
	}
	if !VerifyTOTPCode(record.Secret, code, time.Now()) {
		return nil, ErrInvalidTwoFactorCode
	}
	codes, err := GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = HashRecoveryCode(c)
	}
	updated := *record
	updated.RecoveryCodeHashes = hashes
	if err := a.store.Save(ctx, email, updated); err != nil {
		return nil, err
	}
	a.setCache(email, &updated)
	return codes, nil
}

// RecoveryCodesRemaining reports how many unused recovery codes email has,
// or 0 if 2FA is not enabled.
func (a *TwoFactorAuthenticator) RecoveryCodesRemaining(ctx context.Context, email string) int {
	record, err := a.load(ctx, email)
	if err != nil || record == nil {
		return 0
	}
	return len(record.RecoveryCodeHashes)
}
