package email

import "errors"

// AppPasswordLength is the number of characters in a Gmail app password once
// the display spacing Google's UI inserts has been stripped.
const AppPasswordLength = 16

// Credentials is a Gmail sender identity as an admin enters it. It is
// write-only above the store: nothing in this package or above ever reads an
// AppPassword back out for display.
type Credentials struct {
	Address     string
	AppPassword string
}

// Settings is what the admin-facing API is allowed to know about a stored
// configuration: whether one exists, and the address it sends as.
type Settings struct {
	Configured bool
	Address    string
}

// Message is one outbound email. There is no From: the sender identity comes
// from the stored Credentials, never from the caller.
type Message struct {
	To      string
	Subject string
	Body    string
}

var (
	// ErrNotConfigured is returned by every operation that needs a stored
	// credential when none has been saved yet.
	ErrNotConfigured = errors.New("email: not configured")
	// ErrInvalidAddress means the given address failed net/mail parsing or
	// carried a display name rather than a bare envelope address.
	ErrInvalidAddress = errors.New("email: invalid address")
	// ErrInvalidAppPassword means the password, once whitespace is stripped,
	// is not exactly AppPasswordLength characters.
	ErrInvalidAppPassword = errors.New("email: invalid app password")
	// ErrInvalidRecipient means the test-send recipient failed address
	// validation.
	ErrInvalidRecipient = errors.New("email: invalid recipient")
	// ErrVerificationFailed wraps the cause returned by the sender's Verify
	// call.
	ErrVerificationFailed = errors.New("email: verification failed")
	// ErrSendFailed wraps the cause returned by the sender's Send call.
	ErrSendFailed = errors.New("email: send failed")
)
