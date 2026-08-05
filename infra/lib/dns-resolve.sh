# Public DNS resolution for the installer's pre-flight check.
#
# Resolve A records the way Let's Encrypt will: from public DNS. The local
# resolver is deliberately bypassed because /etc/hosts wins there, and Ubuntu
# maps the machine's own FQDN to 127.0.1.1 — exactly the case when the Remote
# hostname is also the server's hostname (a provider's default name such as
# srvNNNN.hstgr.cloud, or a box named after its domain). Asking the local
# stack made those installs fail a check their DNS actually passed.
#
# DNS-over-HTTPS keeps this dependency-free: curl is already required, while
# dig/host live in a package a minimal Ubuntu image does not ship.

# resolve_public_a <hostname>
# Prints each public A record on its own line; prints nothing when the name
# does not resolve. Falls back to the local stack only when no public
# resolver is reachable, dropping loopback answers that could only have come
# from /etc/hosts.
resolve_public_a() {
    local name="$1" body=""
    for doh in https://cloudflare-dns.com/dns-query https://dns.google/resolve; do
        body=$(curl -fsSL --max-time 8 -H 'accept: application/dns-json' \
            "${doh}?name=${name}&type=A" 2>/dev/null) || continue
        # A reply carrying Status is authoritative for our purposes even when
        # it lists no addresses (NXDOMAIN). Don't fall through to the next
        # resolver, or a genuinely missing record looks like a network blip.
        printf '%s' "$body" | grep -q '"Status"' || continue
        printf '%s' "$body" \
            | grep -oE '"data"[[:space:]]*:[[:space:]]*"([0-9]{1,3}\.){3}[0-9]{1,3}"' \
            | grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' \
            | sort -u
        return 0
    done
    getent ahostsv4 "$name" 2>/dev/null | awk '{print $1}' | grep -vE '^127\.' | sort -u
    # "No addresses" is a valid result, not an error: the filtering greps exit
    # non-zero when they match nothing, which would abort callers running
    # under `set -e`.
    return 0
}
