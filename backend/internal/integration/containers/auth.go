package containers

// Generic credential pipeline. The host holds the canonical copy of each
// profile's files; the client pushes them into containers
// before use and pulls any rotations back afterwards so the host stays
// current. Provider packages own every path and credential rule.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

const authPushTimeout = 30 * time.Second

// EnsureRegisteredCredentials seeds every profile that opts into launch-time
// credential provisioning. Errors are joined so one provider does not hide
// another provider's failure.
func (c *Client) EnsureRegisteredCredentials(ctx context.Context, containerName string) error {
	var errs []error
	for _, profile := range c.AgentProfiles() {
		credentials := profile.Credentials
		if credentials.Empty() || !credentials.SeedOnLaunch {
			continue
		}
		if err := c.EnsureCredentials(ctx, containerName, credentials); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", credentials.Name, err))
		}
	}
	return errors.Join(errs...)
}

// EnsureCredentials pushes a profile's credential files into the container.
// Each file is only pushed when its host mtime is newer than the container
// copy, so this is cheap to call on every prompt.
func (c *Client) EnsureCredentials(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return c.ensureCredentialDirectory(ctx, containerName, spec)
	}
	return c.ensureCredentialFiles(ctx, containerName, spec)
}

func (c *Client) ensureCredentialFiles(ctx context.Context, containerName string, b provisioning.CredentialSpec) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if err := validateCredentialSpec(b); err != nil {
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
		if err := c.pushCredentialFileIfNewer(pctx, f, containerName); err != nil {
			return fmt.Errorf("push %s: %w", f.ContainerPath, err)
		}
	}
	return nil
}

// SyncCredentialsFromContainer pulls credentials back to the host after a
// provider rotates them inside the container.
func (c *Client) SyncCredentialsFromContainer(ctx context.Context, containerName string, spec provisioning.CredentialSpec) error {
	if spec.Directory != nil {
		return c.syncCredentialDirectoryFromContainer(ctx, containerName, spec)
	}
	return c.syncCredentialFilesFromContainer(ctx, containerName, spec)
}

func (c *Client) syncCredentialFilesFromContainer(ctx context.Context, containerName string, b provisioning.CredentialSpec) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if err := validateCredentialSpec(b); err != nil {
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

func validateCredentialSpec(b provisioning.CredentialSpec) error {
	if b.Name == "" {
		return errors.New("auth bundle: Name is required")
	}
	if len(b.Files) == 0 {
		return fmt.Errorf("auth bundle %q: at least one file required", b.Name)
	}
	return nil
}

func (c *Client) pushCredentialFileIfNewer(ctx context.Context, f provisioning.CredentialFile, containerName string) error {
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

func (c *Client) ensureCredentialDirectory(
	ctx context.Context,
	containerName string,
	spec provisioning.CredentialSpec,
) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	directory := spec.Directory
	files, err := regularCredentialFiles(directory.HostPath)
	if err != nil || len(files) == 0 {
		if directory.AllowContainerOnly && c.containerCredentialDirectoryHasFiles(ctx, containerName, directory.ContainerPath) {
			return nil
		}
		if directory.MissingErrorFormat != "" {
			return errors.New(fmt.Sprintf(directory.MissingErrorFormat, containerName))
		}
		return fmt.Errorf("credential directory %s has no files", directory.HostPath)
	}

	pctx, cancel := context.WithTimeout(ctx, authPushTimeout)
	defer cancel()
	for _, path := range directory.ContainerDirs {
		if out, err := c.lxc.Run(pctx, "exec", containerName, "--", "install", "-d", "-m", "700", path); err != nil {
			return fmt.Errorf("mkdir %s in container: %w; output: %s", path, err, out)
		}
	}
	for _, name := range files {
		file := provisioning.CredentialFile{
			HostPath:      filepath.Join(directory.HostPath, name),
			ContainerPath: directory.ContainerPath + "/" + name,
			Mode:          "600",
		}
		if err := c.pushCredentialFileIfNewer(pctx, file, containerName); err != nil {
			return fmt.Errorf("push %s: %w", file.ContainerPath, err)
		}
	}
	return nil
}

func (c *Client) syncCredentialDirectoryFromContainer(
	ctx context.Context,
	containerName string,
	spec provisioning.CredentialSpec,
) error {
	directory := spec.Directory
	if !c.Available() {
		if directory.SyncUnavailableIsNoop {
			return nil
		}
		return errors.New("lxc not available")
	}
	if directory.SyncOnlyWhenHostHasFiles {
		if files, err := regularCredentialFiles(directory.HostPath); err != nil || len(files) == 0 {
			return nil
		}
	}

	pctx, cancel := context.WithTimeout(ctx, authPushTimeout)
	defer cancel()
	out, err := c.lxc.Run(pctx, "exec", containerName, "--",
		"find", directory.ContainerPath, "-maxdepth", "1", "-type", "f", "-printf", "%f\n")
	if err != nil {
		return nil
	}
	for _, name := range strings.Fields(out) {
		containerPath := directory.ContainerPath + "/" + name
		hostPath := filepath.Join(directory.HostPath, name)
		if out, err := c.lxc.Run(pctx, "file", "pull", containerName+containerPath, hostPath); err != nil {
			return fmt.Errorf("pull %s: %w; output: %s", containerPath, err, out)
		}
		_ = os.Chmod(hostPath, 0o600)
		now := time.Now()
		_ = os.Chtimes(hostPath, now, now)
	}
	return nil
}

func (c *Client) containerCredentialDirectoryHasFiles(
	ctx context.Context,
	containerName string,
	containerPath string,
) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.lxc.Run(quickCtx, "exec", containerName, "--",
		"sh", "-c", "ls -1 "+containerPath+" 2>/dev/null | head -1")
	return err == nil && strings.TrimSpace(out) != ""
}

func regularCredentialFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}
