package image

// Pre-flight for the one network failure that is invisible until it is
// expensive. Docker sets the host's iptables FORWARD policy to DROP and
// accepts only its own bridges, which leaves LXD containers with no IPv4
// egress. Docker does not touch ip6tables, so a stranded container still
// reaches everything that publishes an AAAA record: apt, NodeSource, the npm
// registry and Google's CDN all succeed, and the build only dies on the first
// IPv4-only host it needs — github.com, four stages and several minutes in,
// reported as a bare connection timeout with no hint at the cause.
//
// Probing once, before any stage runs, turns that into an immediate and
// actionable error. See infra/lib/container-forwarding.sh for the fix the
// installer applies.

// ipv4EgressProbe opens a TCP connection to well-known IPv4-only resolver
// endpoints. Bash's /dev/tcp keeps this dependency-free, which matters because
// the probe runs before the install script has added anything to the rootfs.
const ipv4EgressProbe = `for endpoint in 1.1.1.1 9.9.9.9 8.8.8.8; do
    timeout 8 bash -c "exec 3<>/dev/tcp/${endpoint}/443" 2>/dev/null && exit 0
done
exit 1`

// ipv4EgressHint explains the failure in the terms an operator can act on,
// rather than the terms the probe failed in.
const ipv4EgressHint = `the builder container cannot reach any IPv4 address.

This is usually Docker: it sets the host's iptables FORWARD policy to DROP and
allows only its own bridges, so LXD containers lose IPv4 while IPv6 keeps
working — which is why apt and npm succeed and only IPv4-only hosts fail.

Fix it by re-running infra/install.sh, which configures this automatically, or
by hand:
  iptables -I DOCKER-USER -i lxdbr0 -j ACCEPT
  iptables -I DOCKER-USER -o lxdbr0 -j ACCEPT`
