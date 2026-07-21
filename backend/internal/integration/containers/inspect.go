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

type containerStateReader interface {
	State(context.Context, string) (serviceproject.ContainerState, error)
}

// containerInspector owns the best-effort snapshot assembled from LXD, the
// guest operating system, configured agent profiles, and host credential files.
type containerInspector struct {
	lxc         CommandRunner
	profiles    *profileRegistry
	states      containerStateReader
	lxd         containerLXDInspector
	guest       containerGuestInspector
	agents      containerAgentInspector
	credentials containerCredentialInspector
}

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

func (c *Client) Inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	return c.inspector.inspect(ctx, containerName)
}

func (i *containerInspector) inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	out := serviceproject.ContainerInspect{Name: containerName}

	state, err := i.states.State(ctx, containerName)
	if err != nil {
		return out, err
	}
	out.State = state
	if state == serviceproject.ContainerStateMissing {
		return out, nil
	}

	i.lxd.inspectConfiguration(ctx, containerName, &out)

	if state == serviceproject.ContainerStateRunning {
		i.lxd.inspectRuntime(ctx, containerName, &out)
		osInfo, disks := i.guest.inspect(ctx, containerName)
		out.OS = osInfo
		out.Disks = disks
		out.SetAgentStatuses(i.agents.inspect(ctx, containerName))
	}

	out.AuthBundles = i.credentials.inspect(ctx, containerName, state)
	return out, nil
}

func (i *containerInspector) queryInstance(ctx context.Context, name string) (*instanceConfig, error) {
	raw, err := i.runQuick(ctx, "query", "/1.0/instances/"+name)
	if err != nil {
		return nil, err
	}
	var cfg instanceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (i *containerInspector) queryInstanceState(ctx context.Context, name string) (*instanceState, error) {
	raw, err := i.runQuick(ctx, "query", "/1.0/instances/"+name+"/state")
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
func (i *containerInspector) probeInContainer(ctx context.Context, name string) (*serviceproject.OSInfo, []serviceproject.DiskUsage) {
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
	raw, err := i.runQuick(ctx, "exec", name, "--", "sh", "-c", script)
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

func (i *containerInspector) inspectAgents(ctx context.Context, containerName string) []serviceproject.AgentContainerStatus {
	profiles := i.profiles.snapshot()
	statuses := make([]serviceproject.AgentContainerStatus, 0, len(profiles))
	for _, profile := range profiles {
		status := serviceproject.AgentContainerStatus{ID: profile.ID}
		if _, err := i.runQuick(ctx, "exec", containerName, "--", "which", profile.CLI.Binary); err == nil {
			status.Installed = true
			if version, err := i.runQuick(ctx, "exec", containerName, "--", profile.CLI.Binary, "--version"); err == nil {
				status.Version = strings.TrimSpace(version)
			}
		}
		if profile.Instructions != nil {
			if _, err := i.runQuick(ctx, "exec", containerName, "--", "test", "-f", profile.Instructions.Path); err == nil {
				status.InstructionsInstalled = true
			}
			if hash, err := i.runQuick(ctx, "exec", containerName, "--", "cat", profile.Instructions.HashPath); err == nil {
				status.InstructionsInSync = strings.TrimSpace(hash) == templateHash(agentInstructionsTemplate)
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (i *containerInspector) inspectAuthBundles(ctx context.Context, containerName string, state serviceproject.ContainerState) []serviceproject.AuthBundleStatus {
	profiles := i.profiles.snapshot()
	out := make([]serviceproject.AuthBundleStatus, 0, len(profiles))
	for _, profile := range profiles {
		b := profile.Credentials
		if len(b.Files) == 0 {
			continue
		}
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
				if raw, err := i.runQuick(ctx,
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

func (i *containerInspector) runQuick(parent context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, inspectQuickTimeout)
	defer cancel()
	return i.lxc.Run(ctx, args...)
}
