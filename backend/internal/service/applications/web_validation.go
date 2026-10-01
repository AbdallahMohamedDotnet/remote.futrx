package applications

import "regexp"

var webSubdomainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidWebSubdomain accepts one DNS label, or empty for existing unnamed apps.
func ValidWebSubdomain(label string) bool { return label == "" || webSubdomainLabel.MatchString(label) }
