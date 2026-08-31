package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileauth"
)

// runSetupToken reissues the first-boot setup token and prints it. It is
// reachable only from the server's own terminal, which is what keeps it from
// reopening the hole it exists to close: nothing web-facing can mint a token
// or ask for the current one to be shown again.
//
// Issuing rotates, so whatever was printed before stops working. That is the
// recovery path for an operator who lost the terminal or let the token expire.
func runSetupToken(ctx context.Context, dataDir, baseURL string, out io.Writer) error {
	store := fileauth.New(dataDir)
	credential, err := store.LocalAdmin(ctx)
	if err != nil {
		return fmt.Errorf("read local admin credential: %w", err)
	}
	if credential != nil {
		return errors.New(
			"this server already has a local administrator; " +
				"remove DATA_DIR/local-admin.json on the host to start setup over",
		)
	}

	issuer := serviceauth.NewSetupTokenIssuer(store)
	token, err := issuer.Issue(ctx)
	if err != nil {
		return err
	}
	// The fragment keeps the token out of the request line, so it never
	// reaches a proxy access log.
	fmt.Fprintf(out,
		"First-time setup required.\n  visit:   %s/#token=%s\n  expires: %s from now\n",
		strings.TrimRight(baseURL, "/"), token, issuer.TTL(),
	)
	return nil
}
