package codex

import (
	"context"
	"errors"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
)

const (
	hostCodexDir  = "/root/.codex"
	hostCodexAuth = "/root/.codex/auth.json"

	containerCodexDir          = "/root/.codex"
	containerCodexAuth         = "/root/.codex/auth.json"
	containerCodexInstructions = "/root/.codex/AGENTS.md"
	containerInstructionsHash  = "/root/.claude/.agents-md.sha256"
	workspaceCodexHome         = "/workspace/.codex"
	containerCodexSkills       = "/root/.codex/skills"
)

var (
	ErrCodexAPIKeyAuth = errors.New("Codex is logged in with an API key; run codex login with ChatGPT to use subscription limits")

	codexProfile = provisioning.Profile{
		ID: string(agent.ProviderCodex),
		CLI: provisioning.CLISpec{
			Name:               "Codex",
			ImageLabel:         "codex",
			Binary:             "codex",
			PackageName:        "@openai/codex",
			Version:            provisioning.MustCLIVersion("CODEX_CLI_VERSION"),
			CheckVersion:       true,
			VerifyAfterInstall: true,
			ReportVersion:      true,
			InstallMode:        provisioning.InstallWithNPM,
			InstallTimeout:     5 * time.Minute,
			WaitTimeout:        2 * time.Minute,
		},
		Credentials: provisioning.CredentialSpec{
			Name:         "codex",
			HostDir:      hostCodexDir,
			ContainerDir: containerCodexDir,
			Files: []provisioning.CredentialFile{
				{
					HostPath:      hostCodexAuth,
					ContainerPath: containerCodexAuth,
					Mode:          "600",
					PushRequired:  true,
					PullRequired:  true,
				},
			},
			SeedOnLaunch: true,
		},
		Instructions: &provisioning.InstructionTarget{
			Path:     containerCodexInstructions,
			HashPath: containerInstructionsHash,
		},
		WorkspaceSkills: &provisioning.WorkspaceSkills{
			WorkspaceHome: workspaceCodexHome,
			HomeSkillsDir: containerCodexSkills,
		},
	}
)

// Profile returns Codex's complete container-facing provisioning policy.
func Profile() provisioning.Profile {
	return codexProfile.Clone()
}

func (p *Provider) ensureCredentials(ctx context.Context, containerName string) error {
	if codexAuthUsesAPIKey(p.profile.Credentials.Files[0].HostPath) {
		return ErrCodexAPIKeyAuth
	}
	return p.containers.EnsureCredentials(ctx, containerName, p.profile.Credentials)
}

func (p *Provider) syncCredentialsFromContainer(ctx context.Context, containerName string) error {
	if err := p.containers.SyncCredentialsFromContainer(ctx, containerName, p.profile.Credentials); err != nil {
		return err
	}
	if codexAuthUsesAPIKey(p.profile.Credentials.Files[0].HostPath) {
		return ErrCodexAPIKeyAuth
	}
	return nil
}

func codexAuthUsesAPIKey(path string) bool {
	_, usesAPIKey := codexAuthMode(path)
	return usesAPIKey
}
