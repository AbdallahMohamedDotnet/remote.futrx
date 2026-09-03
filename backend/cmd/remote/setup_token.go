package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	service "github.com/futrx-com/remote.futrx.com/internal/service"
	serviceuser "github.com/futrx-com/remote.futrx.com/internal/service/user"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileauth"
	"github.com/futrx-com/remote.futrx.com/internal/stores/filesessions"
	"github.com/futrx-com/remote.futrx.com/internal/stores/filetwofactor"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileusers"
)

// unsignedAuthStore is the auth store as this command needs it: it reads the
// credential and the token record, but never mints a session key.
//
// NewAuth asks its store for one, and fileauth creates session.key when it is
// missing. This command signs nothing, so that file would be pure side effect -
// and an operator running the command under sudo before the service has ever
// started would leave the signing key owned by root, which the service cannot
// read and refuses to start without.
type unsignedAuthStore struct {
	*fileauth.Store
}

// SessionKey returns a throwaway key. It is never used to sign or verify
// anything here; auth.New only requires it to be non-empty.
func (unsignedAuthStore) SessionKey(context.Context) ([]byte, error) {
	return make([]byte, 32), nil
}

// runSetupToken reissues the first-boot setup token and prints it. It is
// reachable only from the server's own terminal, which is what keeps it from
// reopening the hole it exists to close: nothing web-facing can mint a token
// or ask for the current one to be shown again.
//
// Issuing rotates, so whatever was printed before stops working. That is the
// recovery path for an operator who lost the terminal or let the token expire.
//
// It asks the auth service whether setup is still gated rather than deciding
// for itself, so this command and the running server can never disagree about
// when a token is worth printing.
func runSetupToken(ctx context.Context, dataDir, baseURL string, authOptions service.AuthOptions, out io.Writer) error {
	authStore := unsignedAuthStore{Store: fileauth.New(dataDir)}
	usersStore, err := fileusers.New(dataDir)
	if err != nil {
		return fmt.Errorf("open user directory: %w", err)
	}
	twoFactorStore, err := filetwofactor.New(dataDir)
	if err != nil {
		return fmt.Errorf("open two-factor store: %w", err)
	}
	sessionRegistryStore, err := filesessions.New(dataDir)
	if err != nil {
		return fmt.Errorf("open session registry store: %w", err)
	}
	auth, err := service.NewAuth(
		ctx,
		authStore,
		serviceuser.New(usersStore),
		baseURL,
		twoFactorStore,
		sessionRegistryStore,
		authOptions,
	)
	if err != nil {
		return err
	}

	token, err := auth.EnsureSetupToken(ctx)
	if err != nil {
		return err
	}
	if token == "" {
		if auth.LocalAdminConfigured() {
			return errors.New(
				"this server already has a local administrator; " +
					"remove DATA_DIR/local-admin.json on the host to start setup over",
			)
		}
		return errors.New(
			"this server already has an administrator, who sets the local password " +
				"themselves from Settings after signing in; no setup token is used",
		)
	}

	announceSetupTokenLink(out, baseURL, token, auth.SetupTokenTTL())
	return nil
}
