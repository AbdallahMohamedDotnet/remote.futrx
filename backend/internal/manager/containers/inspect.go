package containers

// Inspect gathers a rich debugging snapshot of one project's container.
// Sources, in order:
//   1. `lxc query /1.0/instances/<n>`        — instance config (image, devices, limits)
//   2. `lxc query /1.0/instances/<n>/state`  — live runtime stats (memory, network, cpu)
//   3. `lxc exec <n> -- sh -c "<probe>"`     — OS info + df from inside the container
//   4. host-side stat()                       — per-bundle auth file mtimes
// Each section is best-effort: a stopped or missing container leaves
// dependent fields zero-valued.

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const inspectQuickTimeout = 5 * time.Second

// instanceConfig mirrors the subset of /1.0/instances/<n> we care about.
type instanceConfig struct {
	Architecture string                       `json:"architecture"`
	Type         string                       `json:"type"`
	CreatedAt    string                       `json:"created_at"`
	LastUsedAt   string                       `json:"last_used_at"`
	Config       map[string]string            `json:"config"`
	Devices      map[string]map[string]string `json:"devices"`
}

// instanceState mirrors /1.0/instances/<n>/state.
type instanceState struct {
	PID       int    `json:"pid"`
	Processes int    `json:"processes"`
	Status    string `json:"status"`
	CPU       struct {
		Usage int64 `json:"usage"` // nanoseconds
	} `json:"cpu"`
	Disk map[string]struct {
		Usage int64 `json:"usage"`
		Total int64 `json:"total"`
	} `json:"disk"`
	Memory struct {
		Usage     int64 `json:"usage"`
		UsagePeak int64 `json:"usage_peak"`
		Total     int64 `json:"total"`
		Swap      int64 `json:"swap_usage"`
	} `json:"memory"`
	Network map[string]struct {
		HWAddr   string `json:"hwaddr"`
		HostName string `json:"host_name"`
		MTU      int    `json:"mtu"`
		State    string `json:"state"`
		Type     string `json:"type"`
		Counters struct {
			BytesReceived int64 `json:"bytes_received"`
			BytesSent     int64 `json:"bytes_sent"`
		} `json:"counters"`
		Addresses []struct {
			Family  string `json:"family"`
			Address string `json:"address"`
			Netmask string `json:"netmask"`
			Scope   string `json:"scope"`
		} `json:"addresses"`
	} `json:"network"`
}

func (m *Manager) Inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	out := serviceproject.ContainerInspect{Name: containerName}

	state, err := m.State(ctx, containerName)
	if err != nil {
		return out, err
	}
	out.State = state
	if state == serviceproject.ContainerStateMissing {
		return out, nil
	}

	if cfg, err := m.queryInstance(ctx, containerName); err == nil {
		out.Architecture = cfg.Architecture
		out.Type = cfg.Type
		out.CreatedAt = cfg.CreatedAt
		out.LastUsedAt = cfg.LastUsedAt
		out.BootAutostart = cfg.Config["boot.autostart"] == "true"
		if desc := cfg.Config["image.description"]; desc != "" {
			out.Image = desc
		} else if alias := cfg.Config["image.alias"]; alias != "" {
			out.Image = alias
		}
		if cpu, mem, disk := cfg.Config["limits.cpu"], cfg.Config["limits.memory"], cfg.Config["limits.disk"]; cpu != "" || mem != "" || disk != "" {
			out.Limits = &serviceproject.ContainerLimits{CPU: cpu, Memory: mem, Disk: disk}
		}
		if ws, ok := cfg.Devices["workspace"]; ok {
			out.Workspace = &serviceproject.WorkspaceInfo{
				HostSource:    ws["source"],
				ContainerPath: ws["path"],
			}
		}
	}

	if state == serviceproject.ContainerStateRunning {
		if st, err := m.queryInstanceState(ctx, containerName); err == nil {
			out.PID = st.PID
			out.Resources = &serviceproject.ResourceInfo{
				Processes:          st.Processes,
				MemoryCurrentBytes: st.Memory.Usage,
				MemoryPeakBytes:    st.Memory.UsagePeak,
				MemoryTotalBytes:   st.Memory.Total,
				SwapCurrentBytes:   st.Memory.Swap,
				CPUUsageSeconds:    st.CPU.Usage / 1_000_000_000,
			}
			if root, ok := st.Disk["root"]; ok {
				out.Resources.DiskUsageBytes = root.Usage
			}
			for name, n := range st.Network {
				if name == "lo" {
					continue
				}
				addrs := make([]string, 0, len(n.Addresses))
				for _, a := range n.Addresses {
					addrs = append(addrs, a.Address+"/"+a.Netmask)
				}
				out.Network = append(out.Network, serviceproject.NetworkInterface{
					Name:          name,
					State:         n.State,
					Type:          n.Type,
					HostName:      n.HostName,
					MACAddress:    n.HWAddr,
					MTU:           n.MTU,
					Addresses:     addrs,
					BytesReceived: n.Counters.BytesReceived,
					BytesSent:     n.Counters.BytesSent,
				})
			}
		}

		osInfo, disks := m.probeInContainer(ctx, containerName)
		out.OS = osInfo
		out.Disks = disks
		out.Claude = m.inspectClaude(ctx, containerName)
	}

	out.AuthBundles = m.inspectAuthBundles(ctx, containerName, state)
	return out, nil
}

func (m *Manager) queryInstance(ctx context.Context, name string) (*instanceConfig, error) {
	raw, err := m.runQuick(ctx, "query", "/1.0/instances/"+name)
	if err != nil {
		return nil, err
	}
	var cfg instanceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (m *Manager) queryInstanceState(ctx context.Context, name string) (*instanceState, error) {
	raw, err := m.runQuick(ctx, "query", "/1.0/instances/"+name+"/state")
	if err != nil {
		return nil, err
	}
	var st instanceState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// probeInContainer batches several /proc reads + commands into a single
// `lxc exec sh -c` round-trip to keep latency in the low 100ms range. The
// output is split on `=== KEY ===` markers.
func (m *Manager) probeInContainer(ctx context.Context, name string) (*serviceproject.OSInfo, []serviceproject.DiskUsage) {
	script := `
echo "=== OS_RELEASE ==="
cat /etc/os-release 2>/dev/null
echo "=== KERNEL ==="
uname -r 2>/dev/null
echo "=== HOSTNAME ==="
hostname 2>/dev/null
echo "=== NPROC ==="
nproc 2>/dev/null
echo "=== UPTIME ==="
cat /proc/uptime 2>/dev/null
echo "=== DF ==="
df -P -B1 / /workspace 2>/dev/null
echo "=== END ==="
`
	raw, err := m.runQuick(ctx, "exec", name, "--", "sh", "-c", script)
	if err != nil {
		return nil, nil
	}
	sections := splitSections(raw)

	osInfo := &serviceproject.OSInfo{}
	if rel := sections["OS_RELEASE"]; rel != "" {
		for _, line := range strings.Split(rel, "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osInfo.PrettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}
	osInfo.Kernel = strings.TrimSpace(sections["KERNEL"])
	osInfo.Hostname = strings.TrimSpace(sections["HOSTNAME"])
	if n, err := strconv.Atoi(strings.TrimSpace(sections["NPROC"])); err == nil {
		osInfo.CPUCount = n
	}
	if up := strings.TrimSpace(sections["UPTIME"]); up != "" {
		if i := strings.IndexByte(up, ' '); i > 0 {
			if f, err := strconv.ParseFloat(up[:i], 64); err == nil {
				osInfo.UptimeSec = int64(f)
			}
		}
	}

	var disks []serviceproject.DiskUsage
	for _, line := range strings.Split(sections["DF"], "\n") {
		f := strings.Fields(line)
		// df -P -B1 columns: Filesystem 1B-blocks Used Available Capacity MountedOn
		if len(f) < 6 || f[0] == "Filesystem" {
			continue
		}
		total, _ := strconv.ParseInt(f[1], 10, 64)
		used, _ := strconv.ParseInt(f[2], 10, 64)
		avail, _ := strconv.ParseInt(f[3], 10, 64)
		disks = append(disks, serviceproject.DiskUsage{
			MountPath:  f[5],
			Filesystem: f[0],
			TotalBytes: total,
			UsedBytes:  used,
			AvailBytes: avail,
		})
	}
	return osInfo, disks
}

func splitSections(raw string) map[string]string {
	out := map[string]string{}
	var cur string
	var buf strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
			if cur != "" {
				out[cur] = strings.TrimRight(buf.String(), "\n")
			}
			cur = strings.TrimSuffix(strings.TrimPrefix(line, "=== "), " ===")
			buf.Reset()
			continue
		}
		if cur != "" && cur != "END" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if cur != "" && cur != "END" {
		out[cur] = strings.TrimRight(buf.String(), "\n")
	}
	return out
}

func (m *Manager) inspectClaude(ctx context.Context, containerName string) serviceproject.ClaudeContainerStatus {
	var cs serviceproject.ClaudeContainerStatus
	if _, err := m.runQuick(ctx, "exec", containerName, "--", "which", "claude"); err == nil {
		cs.Installed = true
		if v, err := m.runQuick(ctx, "exec", containerName, "--", "claude", "--version"); err == nil {
			cs.Version = strings.TrimSpace(v)
		}
	}

	if _, err := m.runQuick(ctx, "exec", containerName, "--", "test", "-f", containerClaudeMD); err == nil {
		cs.ClaudeMDInstalled = true
	}
	if got, err := m.runQuick(ctx, "exec", containerName, "--", "cat", containerClaudeMDHash); err == nil {
		cs.ClaudeMDInSync = strings.TrimSpace(got) == claudeMDHash()
	}
	return cs
}

func (m *Manager) inspectAuthBundles(ctx context.Context, containerName string, state serviceproject.ContainerState) []serviceproject.AuthBundleStatus {
	bundles := m.AuthBundles()
	out := make([]serviceproject.AuthBundleStatus, 0, len(bundles))
	for _, b := range bundles {
		st := serviceproject.AuthBundleStatus{Name: b.Name}
		for _, f := range b.Files {
			fs := serviceproject.AuthBundleFileStatus{
				HostPath:      f.HostPath,
				ContainerPath: f.ContainerPath,
			}
			if info, err := os.Stat(f.HostPath); err == nil {
				fs.HostExists = true
				fs.HostMTime = info.ModTime().Unix()
			}
			if state == serviceproject.ContainerStateRunning {
				if raw, err := m.runQuick(ctx,
					"exec", containerName, "--", "stat", "-c", "%Y", f.ContainerPath); err == nil {
					if mt, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); perr == nil {
						fs.ContainerExists = true
						fs.ContainerMTime = mt
					}
				}
			}
			switch {
			case fs.HostExists && fs.ContainerExists:
				fs.HostNewer = fs.HostMTime > fs.ContainerMTime
				fs.ContainerNewer = fs.ContainerMTime > fs.HostMTime
			case fs.HostExists && !fs.ContainerExists && state == serviceproject.ContainerStateRunning:
				fs.HostNewer = true
			case !fs.HostExists && fs.ContainerExists:
				fs.ContainerNewer = true
			}
			st.Files = append(st.Files, fs)
		}
		out = append(out, st)
	}
	return out
}

func (m *Manager) runQuick(parent context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, inspectQuickTimeout)
	defer cancel()
	return m.lxc.Run(ctx, args...)
}
