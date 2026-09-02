package minimax

import _ "embed"

const (
	miniMaxLabel              = "MiniMax"
	miniMaxModel              = "MiniMax-M3"
	miniMaxModelContextWindow = 1_000_000
	miniMaxAPIBaseURL         = "https://api.minimax.io/v1"
	miniMaxAPIKeyEnvironment  = "MINIMAX_API_KEY"
	miniMaxWireAPI            = "responses"
	miniMaxReasoningDisabled  = "none"
	miniMaxReasoningAdaptive  = "high"
	miniMaxCLIName            = "MiniMax (Codex harness)"
	miniMaxImageLabel         = "minimax"
	miniMaxCredentialName     = "minimax"
	miniMaxPersistentDevice   = "minimax-home"
	miniMaxHostDirectory      = "minimax"

	containerMiniMaxHome         = "/root/.minimax"
	containerMiniMaxCatalog      = containerMiniMaxHome + "/model-catalog.json"
	containerMiniMaxCatalogHash  = containerMiniMaxHome + "/.model-catalog.sha256"
	containerMiniMaxInstructions = containerMiniMaxHome + "/AGENTS.md"
	containerMiniMaxSkills       = containerMiniMaxHome + "/skills"
	workspaceMiniMaxHome         = "/workspace/.minimax"
)

//go:embed assets/model-catalog.json
var modelCatalog []byte
