package containers

// ListListeners surfaces every externally-reachable TCP listener inside a
// container so the UI's Browser drawer can offer them as a pick list. We
// rely on `ss -tlnHp` (iproute2, always present on the base image) and
// filter out loopback bindings — those are invisible to Caddy because the
// reverse_proxy targets <slug>.lxd:<port>, not 127.0.0.1.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const listenersTimeout = 5 * time.Second

func (m *Manager) ListListeners(ctx context.Context, containerName string) ([]serviceproject.ContainerApp, error) {
	if !m.Available() {
		return nil, errors.New("lxc not available")
	}
	qctx, cancel := context.WithTimeout(ctx, listenersTimeout)
	defer cancel()

	// -t TCP, -l listening, -n numeric, -H no header, -p process info.
	// We accept the inevitable non-zero exit if ss isn't installed (the
	// base image always has iproute2, but a stripped derivative might not).
	out, err := m.lxc.Run(qctx, "exec", containerName, "--", "ss", "-tlnHp")
	if err != nil {
		return nil, fmt.Errorf("ss in container: %w; output: %s", err, out)
	}
	return parseSS(out), nil
}

// parseSS walks `ss -tlnHp` output and emits one ContainerApp per
// externally-reachable port. Duplicate ports (IPv4 + IPv6 of the same
// process) collapse into one entry — we prefer the IPv4 row but if only
// IPv6 exists we keep it.
func parseSS(raw string) []serviceproject.ContainerApp {
	type slot struct {
		app    serviceproject.ContainerApp
		isIPv4 bool
	}
	byPort := map[int]slot{}

	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		// State Recv-Q Send-Q Local-Address:Port Peer-Address:Port [users:...]
		if len(fields) < 4 {
			continue
		}
		host, port, ok := splitListenerAddr(fields[3])
		if !ok {
			continue
		}
		if isLoopback(host) {
			continue
		}

		isV4 := !strings.Contains(host, ":")
		app := serviceproject.ContainerApp{
			Port:    port,
			Address: host,
		}
		if len(fields) >= 6 {
			app.Process, app.PID = parseUsers(strings.Join(fields[5:], " "))
		}

		// Keep one entry per port, preferring IPv4 over IPv6 for nicer
		// display ("0.0.0.0:3000" vs "[::]:3000").
		if cur, exists := byPort[port]; exists {
			if cur.isIPv4 {
				continue
			}
		}
		byPort[port] = slot{app: app, isIPv4: isV4}
	}

	out := make([]serviceproject.ContainerApp, 0, len(byPort))
	for _, s := range byPort {
		out = append(out, s.app)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// splitListenerAddr parses the Local-Address column.
// Accepts "0.0.0.0:3000", "127.0.0.1:53", "127.0.0.53%lo:53", "[::]:22",
// "[::1]:53", "*:3000". Returns the bare host (no brackets, no %iface)
// and the integer port.
func splitListenerAddr(addr string) (string, int, bool) {
	colon := strings.LastIndex(addr, ":")
	if colon < 0 {
		return "", 0, false
	}
	host := addr[:colon]
	portStr := addr[colon+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, false
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if i := strings.Index(host, "%"); i >= 0 {
		host = host[:i]
	}
	if host == "*" {
		host = "0.0.0.0"
	}
	return host, port, true
}

func isLoopback(host string) bool {
	if host == "" {
		return false
	}
	if host == "::1" {
		return true
	}
	return strings.HasPrefix(host, "127.")
}

// parseUsers extracts the first process name + pid from a column like
// `users:(("node",pid=1234,fd=20),("node",pid=1235,fd=21))`.
func parseUsers(s string) (string, int) {
	q := strings.Index(s, "\"")
	if q < 0 {
		return "", 0
	}
	rest := s[q+1:]
	q2 := strings.Index(rest, "\"")
	if q2 < 0 {
		return "", 0
	}
	name := rest[:q2]
	pidIdx := strings.Index(rest, "pid=")
	if pidIdx < 0 {
		return name, 0
	}
	pidStr := rest[pidIdx+4:]
	end := strings.IndexAny(pidStr, ",) ")
	if end < 0 {
		return name, 0
	}
	pid, _ := strconv.Atoi(pidStr[:end])
	return name, pid
}
