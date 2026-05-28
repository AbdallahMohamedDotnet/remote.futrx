package claude

import (
	"encoding/json"
	"strings"
)

// Tool result content can be either a plain string or a list of content blocks
// (text/image). Squash to a string for display.
func normalizeToolResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Otherwise it's a list of content blocks.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var b strings.Builder
		for _, x := range blocks {
			if x.Type == "text" {
				b.WriteString(x.Text)
			}
		}
		return b.String()
	}
	// Fall back to raw JSON.
	return string(raw)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
