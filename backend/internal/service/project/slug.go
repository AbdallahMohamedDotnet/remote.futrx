package project

import (
	"strings"
	"unicode"
)

const MaxSlugLen = 32

// Slugify produces an LXC- and DNS-safe identifier from a display name.
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	lastDash := true
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
	if s == "" {
		s = "project"
	} else if !(s[0] >= 'a' && s[0] <= 'z') {
		s = "p-" + s
	}
	if len(s) > MaxSlugLen {
		s = strings.TrimRight(s[:MaxSlugLen], "-")
	}
	return s
}

func ValidSlug(s string) bool {
	if s == "" || len(s) > MaxSlugLen {
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
