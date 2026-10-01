package httptransport

import (
	"net"
	"regexp"
	"strings"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
)

var applicationInstanceLabel = regexp.MustCompile(`^[a-f0-9]{12}$`)

// ApplicationHost uses a project slug for named apps. Unnamed apps retain their
// existing installation origin until they opt into a manifest subdomain.
func ApplicationHost(identity, subdomain, publicHost string) string {
	if publicHost == "" || !svc.ValidWebSubdomain(subdomain) {
		return ""
	}
	if subdomain != "" {
		if identity == "" || !svc.ValidWebSubdomain(identity) || len(subdomain)+2+len(identity) > 63 {
			return ""
		}
		return subdomain + "--" + identity + "." + publicHost
	}
	if !applicationInstanceLabel.MatchString(identity) {
		return ""
	}
	return identity + ".apps." + publicHost
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
	labels := strings.Split(prefix, ".")
	// Keep the existing preview and built-in editor namespaces with their handlers.
	last := labels[len(labels)-1]
	return last == "apps" || (last != "dev" && last != "code")
}

// ApplicationProject returns the manifest label and project slug of a named host.
func ApplicationProject(host, publicHost string) (string, string, bool) {
	if !IsApplicationHost(host, publicHost) {
		return "", "", false
	}
	name := strings.TrimSuffix(requestHostname(host), "."+requestHostname(publicHost))
	label, slug, found := strings.Cut(name, "--")
	if !found || label == "" || slug == "" || len(name) > 63 || !svc.ValidWebSubdomain(label) || !svc.ValidWebSubdomain(slug) {
		return "", "", false
	}
	return label, slug, true
}

func ApplicationInstanceID(host, publicHost string) (string, bool) {
	suffix := ".apps." + requestHostname(publicHost)
	host = requestHostname(host)
	if publicHost == "" || !strings.HasSuffix(host, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(host, suffix)
	return id, applicationInstanceLabel.MatchString(id)
}

func MatchesApplicationHost(host, identity, subdomain, publicHost string) bool {
	canonical := ApplicationHost(identity, subdomain, publicHost)
	return canonical != "" && requestHostname(host) == requestHostname(canonical)
}
