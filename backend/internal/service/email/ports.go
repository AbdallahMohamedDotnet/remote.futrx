package email

import "context"

// Store persists the single server-wide Gmail credential. Credentials
// returns (nil, nil) when nothing has ever been saved - that absence is the
// correct "not configured" state, not an error, mirroring
// service/auth.TwoFactorStore.Get.
type Store interface {
	Credentials(ctx context.Context) (*Credentials, error)
	Save(ctx context.Context, creds Credentials) error
	Delete(ctx context.Context) error
}

// Sender speaks the outbound protocol. The composition layer supplies the
// real SMTP client; tests supply a recorder.
type Sender interface {
	Verify(ctx context.Context, creds Credentials) error
	Send(ctx context.Context, creds Credentials, msg Message) error
}
