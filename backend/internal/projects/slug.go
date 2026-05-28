package projects

import (
	"strings"
	"unicode"
)

const (
	maxSlugLen = 32
)

// Slugify produces an LXC- and DNS-safe identifier from a display name.
//
// Rules (intersection of LXC container names and DNS labels):
//   - lowercase ASCII letters, digits, hyphens only
//   - must start with a letter
//   - max 32 chars
//
// Collision handling lives in the store layer (suffixes -2, -3 …).
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	lastDash := true // skip leading dashes
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.' || r == '/' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	s := strings.TrimRight(b.String(), "-")
	// LXC names must start with a letter.
	if s == "" {
		s = "project"
	} else if !(s[0] >= 'a' && s[0] <= 'z') {
		s = "p-" + s
	}
	if len(s) > maxSlugLen {
		s = strings.TrimRight(s[:maxSlugLen], "-")
	}
	return s
}

// ValidSlug returns true if a slug is well-formed (used to validate user input
// when the API exposes "rename slug" later — currently always derived).
func ValidSlug(s string) bool {
	if s == "" || len(s) > maxSlugLen {
		return false
	}
	if !(s[0] >= 'a' && s[0] <= 'z') {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}
