package antigravity

import (
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

// Parser satisfies the provider contract for line-oriented callers. agy print
// mode emits unstructured text, so every line is an assistant delta. The
// provider's own run loop streams raw chunks instead (preserving blank lines
// that line scanners drop); this parser exists for the interface and tests.
type Parser struct {
	req agent.RunRequest
}

func NewParser(req agent.RunRequest) *Parser {
	if req.Provider == "" {
		req.Provider = agent.ProviderAntigravity
	}
	return &Parser{req: req}
}

func (p *Parser) ParseLine(line []byte) ([]agent.Event, error) {
	if len(line) == 0 {
		return nil, nil
	}
	return []agent.Event{{
		T:              time.Now().UnixMilli(),
		Type:           agent.EventAssistantTextDelta,
		Provider:       agent.ProviderAntigravity,
		ConversationID: p.req.ConversationID,
		ItemKind:       agent.ItemMessage,
		Text:           string(line) + "\n",
	}}, nil
}
