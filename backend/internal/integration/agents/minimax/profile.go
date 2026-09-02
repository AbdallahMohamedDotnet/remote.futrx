package minimax

import (
	_ "embed"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	"github.com/futrx-com/remote.futrx.com/internal/integration/agents/codexharness"
)

const (
	containerMiniMaxHome         = "/root/.minimax"
	containerMiniMaxCatalog      = containerMiniMaxHome + "/model-catalog.json"
	containerMiniMaxCatalogHash  = containerMiniMaxHome + "/.model-catalog.sha256"
	containerMiniMaxInstructions = containerMiniMaxHome + "/AGENTS.md"
	containerMiniMaxSkills       = containerMiniMaxHome + "/skills"
	workspaceMiniMaxHome         = "/workspace/.minimax"
)

//go:embed assets/model-catalog.json
var modelCatalog []byte

var miniMaxProfile = provisioning.Profile{
	ID:          string(agent.ProviderMiniMax),
	CLI:         codexharness.NewCLISpec("MiniMax (Codex harness)", "minimax"),
	Credentials: provisioning.CredentialSpec{Name: "minimax"},
	PersistentState: []provisioning.PersistentDirectory{{
		Device:        "minimax-home",
		HostDirectory: "minimax",
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
