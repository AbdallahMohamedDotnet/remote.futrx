package containers

import (
	"context"
	"strconv"
	"strings"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// containerGuestInspector owns the batched in-container OS and disk probe.
type containerGuestInspector struct {
	commands *quickCommandRunner
}

func (i *containerGuestInspector) inspect(ctx context.Context, name string) (*serviceproject.OSInfo, []serviceproject.DiskUsage) {
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
	raw, err := i.commands.run(ctx, "exec", name, "--", "sh", "-c", script)
	if err != nil {
		return nil, nil
	}
	sections := splitSections(raw)

	osInfo := &serviceproject.OSInfo{}
	if release := sections["OS_RELEASE"]; release != "" {
		for _, line := range strings.Split(release, "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osInfo.PrettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}
	osInfo.Kernel = strings.TrimSpace(sections["KERNEL"])
	osInfo.Hostname = strings.TrimSpace(sections["HOSTNAME"])
	if count, err := strconv.Atoi(strings.TrimSpace(sections["NPROC"])); err == nil {
		osInfo.CPUCount = count
	}
	if uptime := strings.TrimSpace(sections["UPTIME"]); uptime != "" {
		if separator := strings.IndexByte(uptime, ' '); separator > 0 {
			if seconds, err := strconv.ParseFloat(uptime[:separator], 64); err == nil {
				osInfo.UptimeSec = int64(seconds)
			}
		}
	}

	var disks []serviceproject.DiskUsage
	for _, line := range strings.Split(sections["DF"], "\n") {
		fields := strings.Fields(line)
		// df -P -B1 columns: Filesystem 1B-blocks Used Available Capacity MountedOn
		if len(fields) < 6 || fields[0] == "Filesystem" {
			continue
		}
		total, _ := strconv.ParseInt(fields[1], 10, 64)
		used, _ := strconv.ParseInt(fields[2], 10, 64)
		available, _ := strconv.ParseInt(fields[3], 10, 64)
		disks = append(disks, serviceproject.DiskUsage{
			MountPath:  fields[5],
			Filesystem: fields[0],
			TotalBytes: total,
			UsedBytes:  used,
			AvailBytes: available,
		})
	}
	return osInfo, disks
}
