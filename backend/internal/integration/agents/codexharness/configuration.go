// Package codexharness owns the shared Codex CLI and app-server mechanics used
// by provider adapters that supply their own identity, configuration, and
// execution environment.
package codexharness

import (
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

const (
	codexBinary         = "codex"
	codexPackage        = "@openai/codex"
	codexInstallTimeout = 5 * time.Minute
	codexWaitTimeout    = 2 * time.Minute
)

// NewCLISpec returns the shared Codex CLI installation policy used by
// providers backed by the Codex app-server protocol.
func NewCLISpec(name, imageLabel string) provisioning.CLISpec {
	return provisioning.CLISpec{
		Name:               name,
		ImageLabel:         imageLabel,
		Binary:             codexBinary,
		VersionArgs:        []string{"--version"},
		PackageName:        codexPackage,
		Version:            provisioning.MustCLIVersion("CODEX_CLI_VERSION"),
		CheckVersion:       true,
		VerifyAfterInstall: true,
		ReportVersion:      true,
		InstallMode:        provisioning.InstallWithNPM,
		InstallTimeout:     codexInstallTimeout,
		WaitTimeout:        codexWaitTimeout,
	}
}

// AppServerArgs builds the shared app-server and optional Browser MCP
// arguments around provider-specific Codex configuration arguments.
func AppServerArgs(providerConfig []string, enableBrowser bool) []string {
	args := make([]string, 1, 1+len(providerConfig)+4)
	args[0] = "app-server"
	args = append(args, providerConfig...)
	if enableBrowser {
		args = append(args,
			"-c", `mcp_servers.browser.command="npx"`,
			"-c", `mcp_servers.browser.args=["@playwright/mcp","--cdp-endpoint","http://127.0.0.1:9222","--caps=vision"]`,
		)
	}
	return args
}
