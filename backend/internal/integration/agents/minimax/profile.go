package minimax

import (
	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codexharness"
)

var miniMaxProfile = provisioning.Profile{
	ID:          string(agent.ProviderMiniMax),
	CLI:         codexharness.NewCLISpec(miniMaxCLIName, miniMaxImageLabel),
	Credentials: provisioning.CredentialSpec{Name: miniMaxCredentialName},
	PersistentState: []provisioning.PersistentDirectory{{
		Device:        miniMaxPersistentDevice,
		HostDirectory: miniMaxHostDirectory,
		ContainerPath: containerMiniMaxHome,
	}},
	Instructions: &provisioning.InstructionTarget{
		Path:     containerMiniMaxInstructions,
		HashPath: containerMiniMaxHome + "/.agents-md.sha256",
	},
	WorkspaceSkills: &provisioning.WorkspaceSkills{
		WorkspaceHome: workspaceMiniMaxHome,
		HomeSkillsDir: containerMiniMaxSkills,
	},
	RuntimeAssets: []provisioning.RuntimeAsset{{
		Content:       modelCatalog,
		Path:          containerMiniMaxCatalog,
		HashPath:      containerMiniMaxCatalogHash,
		Mode:          "644",
		Directory:     containerMiniMaxHome,
		DirectoryMode: "700",
	}},
}

// Profile returns MiniMax's isolated Codex runtime policy.
func Profile() provisioning.Profile {
	return miniMaxProfile.Clone()
}
