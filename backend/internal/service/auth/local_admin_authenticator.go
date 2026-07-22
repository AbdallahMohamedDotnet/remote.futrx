package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
)

var localAdminEmailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// LocalAdminAuthenticator owns the local credential and its claim/login
// invariants. Service delegates to it to preserve the public facade.
type LocalAdminAuthenticator struct {
	store LocalAdminStore
	users UserDirectory

	mu         sync.RWMutex
	credential *LocalAdminCredential
	dummyHash  string
	claimMu    sync.Mutex
}

func newLocalAdminAuthenticator(
	store LocalAdminStore,
	users UserDirectory,
	credential *LocalAdminCredential,
) *LocalAdminAuthenticator {
	return &LocalAdminAuthenticator{store: store, users: users, credential: credential}
}

func (a *LocalAdminAuthenticator) setDummyHash(hash string) {
	a.mu.Lock()
	a.dummyHash = hash
	a.mu.Unlock()
}

func (a *LocalAdminAuthenticator) claim(
	ctx context.Context,
	email,
	password,
	authorizedEmail string,
) (User, error) {
	email = normalizeEmail(email)
	if !localAdminEmailPattern.MatchString(email) {
		return User{}, errors.New("valid admin email is required")
	}

	a.claimMu.Lock()
	defer a.claimMu.Unlock()
	a.mu.RLock()
	alreadyClaimed := a.credential != nil
	a.mu.RUnlock()
	if alreadyClaimed {
		return User{}, ErrLocalAdminAlreadyClaimed
	}

	if a.users == nil {
		return User{}, errors.New("users directory is not configured")
	}
	if first, err := a.users.FirstAdmin(ctx); err != nil {
		return User{}, err
	} else if first != nil {
		authorizedEmail = normalizeEmail(authorizedEmail)
		isAdmin, authErr := a.users.IsAdmin(ctx, authorizedEmail)
		if authErr != nil {
			return User{}, authErr
		}
		if !isAdmin || authorizedEmail != email || email != normalizeEmail(first.Email) {
			return User{}, ErrAdminClaimUnauthorized
		}
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	credential := LocalAdminCredential{Email: email, PasswordHash: passwordHash}
	if err := a.store.CreateLocalAdmin(ctx, credential); err != nil {
		return User{}, err
	}
	a.mu.Lock()
	a.credential = &credential
	a.mu.Unlock()

	registered, err := a.users.IsRegistered(ctx, email)
	if err != nil {
		return User{}, err
	}
	if !registered {
		if err := a.users.AddBootstrapAdmin(ctx, email); err != nil {
			return User{}, err
		}
	}
	return localAdminUser(email), nil
}

func (a *LocalAdminAuthenticator) login(email, password string) (User, error) {
	a.mu.RLock()
	credential := a.credential
	hash := a.dummyHash
	if credential != nil {
		copy := *credential
		credential = &copy
		hash = copy.PasswordHash
	}
	a.mu.RUnlock()

	passwordOK, err := VerifyPassword(hash, password)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	emailOK := credential != nil && normalizeEmail(email) == credential.Email
	if !emailOK || !passwordOK {
		return User{}, ErrInvalidCredentials
	}
	return localAdminUser(credential.Email), nil
}

func (a *LocalAdminAuthenticator) configured() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.credential != nil
}

func (a *LocalAdminAuthenticator) isLocalAdmin(email string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.credential != nil && a.credential.Email == normalizeEmail(email)
}

func (a *LocalAdminAuthenticator) adminEmail() (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.credential == nil {
		return "", false
	}
	return a.credential.Email, true
}

func localAdminUser(email string) User {
	return User{Email: normalizeEmail(email), Sub: "local-admin"}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
