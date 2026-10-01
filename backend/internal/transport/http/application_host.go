package httptransport

import (
	"net"
	"regexp"
	"strings"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
)

var previewHostLabel = regexp.MustCompile(`^dev--[a-z0-9][a-z0-9-]*--\d{4,5}$`)

var applicationInstanceLabel = regexp.MustCompile(`^[a-f0-9]{12}$`)

// ApplicationHost puts named and unnamed apps one DNS label below the platform.
// The two separators in unnamed hosts keep them distinct from named app hosts.
func ApplicationHost(identity, subdomain, publicHost string) string {
	if publicHost == "" || !svc.ValidWebSubdomain(subdomain) {
		return ""
	}
	if subdomain != "" {
		if !validProjectApplicationLabel(subdomain, identity) {
			return ""
		}
		return subdomain + "--" + identity + "." + publicHost
	}
	if !applicationInstanceLabel.MatchString(identity) {
		return ""
	}
	return "app--" + identity + "--instance." + publicHost
}

// validProjectApplicationLabel owns the combined app/project DNS-label rules.
func validProjectApplicationLabel(label, slug string) bool {
	return label != "" && slug != "" && len(label)+2+len(slug) <= 63 &&
		svc.ValidWebSubdomain(label) && svc.ValidWebSubdomain(slug)
}

func requestHostname(host string) string {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	// DNS treats a trailing dot as equivalent. Reserve that spelling too so
	// an app hostname can never fall through to the platform path router.
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// IsApplicationHost also reserves malformed names, so they never fall through
// to the platform API, login pages, or static UI on an application origin.
func IsApplicationHost(host, publicHost string) bool {
	host, base := requestHostname(host), requestHostname(publicHost)
	if base == "" || !strings.HasSuffix(host, "."+base) {
		return false
	}
	prefix := strings.TrimSuffix(host, "."+base)
	// Only the built-in launcher and valid preview hosts bypass the app gateway.
	// All other subdomains, including obsolete nested hosts, fail closed here.
	return prefix != "code" && !previewHostLabel.MatchString(prefix)
}

// ApplicationProject returns the manifest label and project slug of a named host.
func ApplicationProject(host, publicHost string) (string, string, bool) {
	if !IsApplicationHost(host, publicHost) {
		return "", "", false
	}
	name := strings.TrimSuffix(requestHostname(host), "."+requestHostname(publicHost))
	label, slug, found := strings.Cut(name, "--")
	if !found || !validProjectApplicationLabel(label, slug) {
		return "", "", false
	}
	return label, slug, true
}

func ApplicationInstanceID(host, publicHost string) (string, bool) {
	suffix := "--instance." + requestHostname(publicHost)
	host = requestHostname(host)
	if publicHost == "" || !strings.HasSuffix(host, suffix) {
		return "", false
	}
	id := strings.TrimPrefix(strings.TrimSuffix(host, suffix), "app--")
	if !strings.HasPrefix(host, "app--") {
		return "", false
	}
	return id, applicationInstanceLabel.MatchString(id)
}

func MatchesApplicationHost(host, identity, subdomain, publicHost string) bool {
	canonical := ApplicationHost(identity, subdomain, publicHost)
	return canonical != "" && requestHostname(host) == requestHostname(canonical)
}
