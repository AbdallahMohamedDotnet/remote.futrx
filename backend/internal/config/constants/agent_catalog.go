package constants

import (
	_ "embed"
	"time"
)

const (
	CodexHarnessBinary         = "codex"
	CodexHarnessPackage        = "@openai/codex"
	CodexHarnessVersionPin     = "CODEX_CLI_VERSION"
	CodexHarnessVersionFlag    = "--version"
	CodexHarnessInstallTimeout = 5 * time.Minute
	CodexHarnessWaitTimeout    = 2 * time.Minute
	CodexHarnessAppServer      = "app-server"
	CodexHarnessBrowserCommand = `mcp_servers.browser.command="npx"`
	CodexHarnessBrowserArgs    = `mcp_servers.browser.args=["@playwright/mcp","--cdp-endpoint","http://127.0.0.1:9222","--caps=vision"]`

	MiniMaxLabel                        = "MiniMax"
	MiniMaxModel                        = "MiniMax-M3"
	MiniMaxModelDescription             = "MiniMax M3 with a 1,000,000-token context window"
	MiniMaxAutoModelLabel               = "MiniMax default"
	MiniMaxModelContextWindow           = 1_000_000
	MiniMaxAPIBaseURL                   = "https://api.minimax.io/v1"
	MiniMaxAPIValidationTimeout         = 10 * time.Second
	MiniMaxAPIKeyEnvironment            = "MINIMAX_API_KEY"
	MiniMaxWireAPI                      = "responses"
	MiniMaxReasoningDisabled            = "none"
	MiniMaxReasoningDisabledLabel       = "Think-Off"
	MiniMaxReasoningDisabledDescription = "Disable Adaptive Thinking"
	MiniMaxReasoningAdaptive            = "high"
	MiniMaxReasoningAdaptiveLabel       = "Adaptive"
	MiniMaxReasoningAdaptiveDescription = "Enable Adaptive Thinking"
	MiniMaxCLIName                      = "MiniMax (Codex harness)"
	MiniMaxImageLabel                   = "minimax"
	MiniMaxCredentialName               = "minimax"
	MiniMaxPersistentDevice             = "minimax-home"
	MiniMaxHostDirectory                = "minimax"
	MiniMaxAuthInstructions             = "Add your MiniMax API key to make MiniMax available in project chats."
	MiniMaxAPIKeyCreateURL              = "https://platform.minimax.io/console/access"

	MiniMaxContainerHome             = "/root/.minimax"
	MiniMaxContainerCatalog          = MiniMaxContainerHome + "/model-catalog.json"
	MiniMaxContainerCatalogHash      = MiniMaxContainerHome + "/.model-catalog.sha256"
	MiniMaxContainerInstructions     = MiniMaxContainerHome + "/AGENTS.md"
	MiniMaxContainerInstructionsHash = MiniMaxContainerHome + "/.agents-md.sha256"
	MiniMaxContainerSkills           = MiniMaxContainerHome + "/skills"
	MiniMaxWorkspaceHome             = "/workspace/.minimax"
)

//go:embed assets/minimax-model-catalog.json
var miniMaxModelCatalog []byte

// MiniMaxModelCatalog returns an independent copy of the compiled-in catalog.
func MiniMaxModelCatalog() []byte {
	return append([]byte(nil), miniMaxModelCatalog...)
}
