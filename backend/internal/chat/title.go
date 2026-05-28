package chat

import (
	"fmt"
	"strings"
	"time"
)

// TitleFromPrompt produces a short summary used when a chat is created with
// no explicit title. First 60 chars of the first prompt, single line.
func TitleFromPrompt(prompt string) string {
	t := strings.TrimSpace(prompt)
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	if len(t) > 60 {
		t = t[:60] + "…"
	}
	if t == "" {
		t = fmt.Sprintf("Chat %s", time.Now().Format("Jan 2 15:04"))
	}
	return t
}
