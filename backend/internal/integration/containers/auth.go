package containers

// Generic OAuth/credential bundle pipeline. The host holds the canonical
// copy of each bundle's files; the client pushes them into containers
// before use and pulls any rotations back afterwards so the host stays
// current. Nothing in this file knows about Claude — bundle definitions
// live with their providers (see claude.go).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const authPushTimeout = 30 * time.Second

// AuthBundle describes one provider's credential set on disk. Register an
// instance via Client.RegisterAuthBundle to have it auto-seeded on every
// fresh container launch; call EnsureAuthBundle / SyncAuthBundleFromContainer
// directly to drive the push/pull pipeline at other points (e.g. before and
// after each prompt).
type AuthBundle struct {
	// Name is a short identifier used in error messages. Example: "claude".
	Name string

	// HostDir, if set, is ensured (mode 0700) before pulling files back from
	// the container. Use for providers whose credentials live in a
	// dedicated dotdir like /root/.claude.
	HostDir string

	// ContainerDir, if set, is created inside the container (mode 0700)
	// before any file is pushed.
	ContainerDir string

	// Files is the set of credential files the bundle owns.
	Files []AuthFile

	// LegacyDevices are container device names removed (best-effort) before
	// each push. Use this to migrate older containers off prior disk-mount
	// auth schemes.
	LegacyDevices []string
}

// AuthFile is one credential file inside a bundle.
type AuthFile struct {
	HostPath      string
	ContainerPath string
	// Mode is the octal mode string passed to `lxc file push --mode=`.
	// Defaults to "600".
	Mode string
	// PushRequired: if true, EnsureAuthBundle errors when the file is
	// missing on the host. Use for the file that gates "is this provider
	// authenticated at all?".
	PushRequired bool
	// PullRequired: if true, SyncAuthBundleFromContainer errors when the
	// file is missing in the container.
	PullRequired bool
}

// RegisterAuthBundle adds a bundle to the seed list used by Launch. Safe to
// call from main during wiring; not intended to be called concurrently with
// Launch.
func (c *Client) RegisterAuthBundle(b AuthBundle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bundles = append(c.bundles, b)
}

// AuthBundles returns a snapshot of the registered bundles. Useful for tests
// and diagnostics.
func (c *Client) AuthBundles() []AuthBundle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]AuthBundle, len(c.bundles))
	copy(out, c.bundles)
	return out
}

// EnsureRegisteredAuth seeds every registered bundle into the container.
// Errors from individual bundles are joined so a single bad bundle doesn't
// hide problems in the others.
func (c *Client) EnsureRegisteredAuth(ctx context.Context, containerName string) error {
	var errs []error
	for _, b := range c.AuthBundles() {
		if err := c.EnsureAuthBundle(ctx, containerName, b); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name, err))
		}
	}
	return errors.Join(errs...)
}

// EnsureAuthBundle pushes the bundle's files into the container. Each file is
// only pushed when its host mtime is newer than the container copy, so this
// is cheap to call on every prompt.
func (c *Client) EnsureAuthBundle(ctx context.Context, containerName string, b AuthBundle) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if err := b.validate(); err != nil {
		return err
	}

	for _, dev := range b.LegacyDevices {
		dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
		_, _ = c.lxc.Run(dctx, "config", "device", "remove", containerName, dev)
		cancelD()
	}

	// Gate: every PushRequired host file must exist before we touch the
	// container further. An unauthenticated provider should not leave
	// half-created dirs behind.
	for _, f := range b.Files {
		if !f.PushRequired {
			continue
		}
		if _, err := os.Stat(f.HostPath); err != nil {
			return fmt.Errorf("host file missing (provider not authenticated yet?): %s", f.HostPath)
		}
	}

	pctx, cancelP := context.WithTimeout(ctx, authPushTimeout)
	defer cancelP()

	if b.ContainerDir != "" {
		if out, err := c.lxc.Run(pctx, "exec", containerName, "--",
			"install", "-d", "-m", "700", b.ContainerDir); err != nil {
			return fmt.Errorf("mkdir %s in container: %w; output: %s",
				b.ContainerDir, err, out)
		}
	}

	for _, f := range b.Files {
		if _, err := os.Stat(f.HostPath); err != nil {
			// PushRequired files were already gated above; only optional
			// files can still be missing here, and we silently skip them.
			continue
		}
		if err := c.pushAuthFileIfNewer(pctx, f, containerName); err != nil {
			return fmt.Errorf("push %s: %w", f.ContainerPath, err)
		}
	}
	return nil
}

// SyncAuthBundleFromContainer pulls the bundle's files back to the host.
// Necessary when the in-container process rotates credentials (OAuth refresh
// tokens, etc.) — without this, the next push would overwrite the rotation
// with stale host data.
func (c *Client) SyncAuthBundleFromContainer(ctx context.Context, containerName string, b AuthBundle) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if err := b.validate(); err != nil {
		return err
	}

	if b.HostDir != "" {
		if err := os.MkdirAll(b.HostDir, 0o700); err != nil {
			return fmt.Errorf("create host dir %s: %w", b.HostDir, err)
		}
		_ = os.Chmod(b.HostDir, 0o700)
	}

	pctx, cancel := context.WithTimeout(ctx, authPushTimeout)
	defer cancel()

	for _, f := range b.Files {
		if out, err := c.lxc.Run(pctx, "exec", containerName, "--", "test", "-f", f.ContainerPath); err != nil {
			if f.PullRequired {
				return fmt.Errorf("container file missing %s: %w; output: %s",
					f.ContainerPath, err, out)
			}
			continue
		}
		if out, err := c.lxc.Run(pctx, "file", "pull", containerName+f.ContainerPath, f.HostPath); err != nil {
			return fmt.Errorf("pull %s: %w; output: %s",
				f.ContainerPath, err, out)
		}
		_ = os.Chmod(f.HostPath, 0o600)
		now := time.Now()
		_ = os.Chtimes(f.HostPath, now, now)
	}
	return nil
}

func (b AuthBundle) validate() error {
	if b.Name == "" {
		return errors.New("auth bundle: Name is required")
	}
	if len(b.Files) == 0 {
		return fmt.Errorf("auth bundle %q: at least one file required", b.Name)
	}
	return nil
}

func (c *Client) pushAuthFileIfNewer(ctx context.Context, f AuthFile, containerName string) error {
	hostInfo, err := os.Stat(f.HostPath)
	if err != nil {
		return err
	}

	shouldPush := true
	if out, err := c.lxc.Run(ctx, "exec", containerName, "--", "stat", "-c", "%Y", f.ContainerPath); err == nil {
		if containerUnix, parseErr := strconv.ParseInt(strings.TrimSpace(out), 10, 64); parseErr == nil {
			shouldPush = hostInfo.ModTime().Unix() > containerUnix
		}
	}
	if !shouldPush {
		return nil
	}

	mode := f.Mode
	if mode == "" {
		mode = "600"
	}
	if out, err := c.lxc.Run(ctx, "file", "push", "--mode="+mode, f.HostPath, containerName+f.ContainerPath); err != nil {
		return fmt.Errorf("lxc file push: %w; output: %s", err, out)
	}
	return nil
}
