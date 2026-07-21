package containers

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	agentCLIInstallTimeout = 5 * time.Minute
	agentCLIWaitTimeout    = 2 * time.Minute
)

//go:embed agent-cli-versions.env
var agentCLIVersionManifest string

var (
	pinnedClaudeCodeVersion = mustAgentCLIVersion("CLAUDE_CODE_VERSION")
	pinnedCodexCLIVersion   = mustAgentCLIVersion("CODEX_CLI_VERSION")

	claudeCLISpec = agentCLISpec{
		name:        "Claude Code",
		binary:      "claude",
		packageName: "@anthropic-ai/claude-code",
		version:     pinnedClaudeCodeVersion,
	}
	codexCLISpec = agentCLISpec{
		name:        "Codex",
		binary:      "codex",
		packageName: "@openai/codex",
		version:     pinnedCodexCLIVersion,
	}
)

type agentCLISpec struct {
	name        string
	binary      string
	packageName string
	version     string
}

func (s agentCLISpec) npmPackage() string {
	return s.packageName + "@" + s.version
}

func mustAgentCLIVersion(key string) string {
	scanner := bufio.NewScanner(strings.NewReader(agentCLIVersionManifest))
	for scanner.Scan() {
		name, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if ok && name == key {
			value = strings.TrimSpace(value)
			if _, valid := parseSemanticVersion(value); valid {
				return value
			}
			panic("invalid " + key + " in agent-cli-versions.env")
		}
	}
	panic("missing " + key + " in agent-cli-versions.env")
}

// ensureAgentCLI is cheap on the normal path (one local `--version` call).
// Missing or stale CLIs are upgraded to the repository pin, and concurrent
// prompt starts coalesce around the npm install already running in the
// container.
func (c *Client) ensureAgentCLI(ctx context.Context, containerName string, spec agentCLISpec) error {
	if !c.Available() {
		return errors.New("lxc not available")
	}
	if c.agentCLIReady(ctx, containerName, spec) {
		return nil
	}
	if c.agentCLIInstallRunning(ctx, containerName, spec) {
		waitCtx, cancel := context.WithTimeout(ctx, agentCLIWaitTimeout)
		defer cancel()
		if err := c.waitForAgentCLI(waitCtx, containerName, spec); err == nil {
			return nil
		}
	}

	installCtx, cancel := context.WithTimeout(ctx, agentCLIInstallTimeout)
	defer cancel()

	var out string
	var err error
	if c.containerCommandExists(installCtx, containerName, "npm") {
		out, err = c.lxc.Run(installCtx, "exec", containerName, "--",
			"npm", "install", "-g", spec.npmPackage(), "--silent")
	} else {
		// Very old containers may pre-date Node/npm. Reuse the full image recipe
		// in that case so the runtime still self-heals from a bare rootfs.
		out, err = c.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", BaseImageInstallScript)
	}
	if err != nil {
		waitCtx, cancelWait := context.WithTimeout(ctx, 90*time.Second)
		defer cancelWait()
		if waitErr := c.waitForAgentCLI(waitCtx, containerName, spec); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install %s %s in %s: %w; output: %s",
			spec.name, spec.version, containerName, err, truncateOut(out, 1000))
	}
	if !c.agentCLIReady(ctx, containerName, spec) {
		return fmt.Errorf("install %s %s in %s completed but the required version is unavailable",
			spec.name, spec.version, containerName)
	}
	return nil
}

func (c *Client) agentCLIReady(ctx context.Context, containerName string, spec agentCLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.lxc.Run(quickCtx, "exec", containerName, "--", spec.binary, "--version")
	return err == nil && semanticVersionAtLeast(out, spec.version)
}

func (c *Client) agentCLIInstallRunning(ctx context.Context, containerName string, spec agentCLISpec) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := c.lxc.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*"+spec.packageName)
	return err == nil && strings.TrimSpace(out) != ""
}

func (c *Client) containerCommandExists(ctx context.Context, containerName, command string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := c.lxc.Run(quickCtx, "exec", containerName, "--", "which", command)
	return err == nil
}

func (c *Client) waitForAgentCLI(ctx context.Context, containerName string, spec agentCLISpec) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if c.agentCLIReady(ctx, containerName, spec) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

var semanticVersionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)(-([0-9A-Za-z.-]+))?`)

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	match := semanticVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return semanticVersion{}, false
	}
	major, errMajor := strconv.Atoi(match[1])
	minor, errMinor := strconv.Atoi(match[2])
	patch, errPatch := strconv.Atoi(match[3])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: match[5]}, true
}

func semanticVersionAtLeast(actualOutput, minimumValue string) bool {
	actual, ok := parseSemanticVersion(actualOutput)
	if !ok {
		return false
	}
	minimum, ok := parseSemanticVersion(minimumValue)
	if !ok {
		return false
	}
	actualCore := [3]int{actual.major, actual.minor, actual.patch}
	minimumCore := [3]int{minimum.major, minimum.minor, minimum.patch}
	for i := range actualCore {
		if actualCore[i] != minimumCore[i] {
			return actualCore[i] > minimumCore[i]
		}
	}
	if actual.prerelease == minimum.prerelease {
		return true
	}
	// A stable release is newer than a prerelease with the same core.
	if actual.prerelease == "" {
		return true
	}
	if minimum.prerelease == "" {
		return false
	}
	return actual.prerelease >= minimum.prerelease
}
