package provisioning

import _ "embed"

//go:embed assets/AGENTS.md
var instructionsTemplate []byte

func InstructionsTemplate() []byte {
	return append([]byte(nil), instructionsTemplate...)
}
