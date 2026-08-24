package webpush

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// ErrUnsafeEndpoint means a subscription attempted to direct the server to a
// local, private, reserved, or otherwise non-public network address.
var ErrUnsafeEndpoint = errors.New("unsafe web push endpoint")

type lookupNetIPFunc func(context.Context, string, string) ([]netip.Addr, error)
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

// publicDialer resolves a hostname once, validates every answer, then dials one
// of those exact IPs. Pinning the dial to the checked result prevents a second
// lookup from turning a public hostname into a private target (DNS rebinding).
type publicDialer struct {
	lookup lookupNetIPFunc
	dial   dialContextFunc
}

func newSafeHTTPClient() *http.Client {
	networkDialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	dialer := publicDialer{
		lookup: net.DefaultResolver.LookupNetIP,
		dial:   networkDialer.DialContext,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A configured HTTP proxy could perform its own unchecked DNS lookup and
	// bypass the pinned dialer, so push delivery always connects directly.
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		// Push endpoints are opaque capabilities, not navigation URLs. Following
		// a redirect would let an otherwise public endpoint redirect into the
		// server's private network.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (d publicDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse web push address: %w", err)
	}

	addresses, err := d.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve web push endpoint %q: no addresses", host)
	}
	// Reject the whole hostname when any answer is unsafe. Selecting only a
	// public answer would leave mixed-answer DNS rebinding tricks available.
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return nil, fmt.Errorf("%w: %s resolves to %s", ErrUnsafeEndpoint, host, address)
		}
	}

	var dialErrors []error
	for _, resolved := range addresses {
		connection, err := d.dial(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, err)
	}
	return nil, fmt.Errorf("dial web push endpoint %q: %w", host, errors.Join(dialErrors...))
}

func (d publicDialer) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address}, nil
	}
	addresses, err := d.lookup(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve web push endpoint %q: %w", host, err)
	}
	return addresses, nil
}

func validateEndpointURL(endpoint string) error {
	if len(endpoint) > 2048 {
		return fmt.Errorf("%w: endpoint is too long", ErrUnsafeEndpoint)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" {
		return errors.New("push endpoint must be an absolute https URL")
	}
	if parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return fmt.Errorf("%w: endpoint contains unsupported URL components", ErrUnsafeEndpoint)
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("%w: localhost is not a push service", ErrUnsafeEndpoint)
	}
	if address, err := netip.ParseAddr(host); err == nil && !isPublicAddress(address) {
		return fmt.Errorf("%w: %s is not public", ErrUnsafeEndpoint, host)
	}
	return nil
}

var nonPublicPrefixes = []netip.Prefix{
	// Shared, benchmarking, documentation, protocol-assignment, and reserved
	// IPv4 ranges that are not valid public push-service destinations.
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	// Documentation, transition, and well-known NAT64 ranges can otherwise
	// hide or synthesize a non-public IPv4 destination.
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

func isPublicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
