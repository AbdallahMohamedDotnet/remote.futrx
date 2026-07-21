package httphandlers

import (
	"bufio"
	"context"
	"math"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

const projectWorkspaceRoot = "/var/lib/remote/projects"

type ServerInfoHandler struct {
	dataDir   string
	startedAt time.Time
	collect   func(context.Context) ServerInfo
}

type ServerInfo struct {
	CollectedAt int64             `json:"collectedAt"`
	Host        ServerHostInfo    `json:"host"`
	CPU         ServerCPUInfo     `json:"cpu"`
	Memory      ServerMemoryInfo  `json:"memory"`
	Storage     ServerStorageInfo `json:"storage"`
	Network     ServerNetworkInfo `json:"network"`
	Process     ServerProcessInfo `json:"process"`
}

type ServerHostInfo struct {
	Hostname         string `json:"hostname"`
	OS               string `json:"os"`
	Platform         string `json:"platform,omitempty"`
	Architecture     string `json:"architecture"`
	Kernel           string `json:"kernel,omitempty"`
	UptimeSec        int64  `json:"uptimeSec,omitempty"`
	BootedAt         int64  `json:"bootedAt,omitempty"`
	ServiceUptimeSec int64  `json:"serviceUptimeSec"`
	GoVersion        string `json:"goVersion"`
	DataPath         string `json:"dataPath"`
	WorkspacePath    string `json:"workspacePath"`
}

type ServerCPUInfo struct {
	LogicalCores  int      `json:"logicalCores"`
	Model         string   `json:"model,omitempty"`
	UsagePercent  *float64 `json:"usagePercent,omitempty"`
	LoadAverage1  *float64 `json:"loadAverage1,omitempty"`
	LoadAverage5  *float64 `json:"loadAverage5,omitempty"`
	LoadAverage15 *float64 `json:"loadAverage15,omitempty"`
}

type ServerMemoryInfo struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	FreeBytes      uint64  `json:"freeBytes"`
	CachedBytes    uint64  `json:"cachedBytes"`
	BuffersBytes   uint64  `json:"buffersBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	SwapTotalBytes uint64  `json:"swapTotalBytes"`
	SwapUsedBytes  uint64  `json:"swapUsedBytes"`
	SwapFreeBytes  uint64  `json:"swapFreeBytes"`
}

type ServerStorageInfo struct {
	TotalBytes     uint64               `json:"totalBytes"`
	UsedBytes      uint64               `json:"usedBytes"`
	AvailableBytes uint64               `json:"availableBytes"`
	UsagePercent   float64              `json:"usagePercent"`
	Mounts         []ServerStorageMount `json:"mounts"`
}

type ServerStorageMount struct {
	Device         string  `json:"device,omitempty"`
	MountPath      string  `json:"mountPath"`
	Filesystem     string  `json:"filesystem,omitempty"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type ServerNetworkInfo struct {
	ReceivedBytes uint64                   `json:"receivedBytes"`
	SentBytes     uint64                   `json:"sentBytes"`
	Interfaces    []ServerNetworkInterface `json:"interfaces"`
}

type ServerNetworkInterface struct {
	Name          string   `json:"name"`
	MTU           int      `json:"mtu,omitempty"`
	HardwareAddr  string   `json:"hardwareAddress,omitempty"`
	Addresses     []string `json:"addresses,omitempty"`
	ReceivedBytes uint64   `json:"receivedBytes"`
	SentBytes     uint64   `json:"sentBytes"`
	Loopback      bool     `json:"loopback"`
	Up            bool     `json:"up"`
}

type ServerProcessInfo struct {
	PID               int    `json:"pid"`
	Goroutines        int    `json:"goroutines"`
	OpenFileHandles   int    `json:"openFileHandles,omitempty"`
	AllocatedBytes    uint64 `json:"allocatedBytes"`
	HeapInUseBytes    uint64 `json:"heapInUseBytes"`
	SystemMemoryBytes uint64 `json:"systemMemoryBytes"`
}

func NewServerInfoHandler(dataDir string) *ServerInfoHandler {
	h := &ServerInfoHandler{dataDir: dataDir, startedAt: time.Now()}
	h.collect = h.collectInfo
	return h
}

func (h *ServerInfoHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/server/info", h.HandleInfo)
}

func (h *ServerInfoHandler) HandleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	httptransport.SendJSON(w, http.StatusOK, h.collect(r.Context()))
}

func (h *ServerInfoHandler) collectInfo(ctx context.Context) ServerInfo {
	now := time.Now()
	uptime := readUptime()
	hostname, _ := os.Hostname()
	mem := readMemoryInfo()
	load1, load5, load15 := readLoadAverage()
	cpuStart, cpuAvailable := readCPUSample()

	info := ServerInfo{
		CollectedAt: now.Unix(),
		Host: ServerHostInfo{
			Hostname:         hostname,
			OS:               runtime.GOOS,
			Platform:         readOSPrettyName(),
			Architecture:     runtime.GOARCH,
			Kernel:           readTrimmedFile("/proc/sys/kernel/osrelease"),
			UptimeSec:        uptime,
			ServiceUptimeSec: int64(now.Sub(h.startedAt).Seconds()),
			GoVersion:        runtime.Version(),
			DataPath:         h.dataDir,
			WorkspacePath:    projectWorkspaceRoot,
		},
		CPU: ServerCPUInfo{
			LogicalCores:  runtime.NumCPU(),
			Model:         readCPUModel(),
			LoadAverage1:  load1,
			LoadAverage5:  load5,
			LoadAverage15: load15,
		},
		Memory:  mem,
		Storage: readStorageInfo(),
		Network: readNetworkInfo(),
		Process: readProcessInfo(),
	}
	if uptime > 0 {
		info.Host.BootedAt = now.Add(-time.Duration(uptime) * time.Second).Unix()
	}

	if cpuAvailable {
		timer := time.NewTimer(120 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return info
		case <-timer.C:
			if cpuEnd, ok := readCPUSample(); ok {
				usage := cpuUsage(cpuStart, cpuEnd)
				info.CPU.UsagePercent = &usage
			}
		}
	}

	return info
}

type cpuSample struct {
	total uint64
	idle  uint64
}

func readCPUSample() (cpuSample, bool) {
	line := firstLine(readTrimmedFile("/proc/stat"))
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, false
	}
	var values []uint64
	for _, raw := range fields[1:] {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuSample{total: total, idle: idle}, true
}

func cpuUsage(start, end cpuSample) float64 {
	if end.total <= start.total {
		return 0
	}
	totalDelta := end.total - start.total
	idleDelta := uint64(0)
	if end.idle > start.idle {
		idleDelta = end.idle - start.idle
	}
	if idleDelta > totalDelta {
		idleDelta = totalDelta
	}
	return roundedPercent(float64(totalDelta-idleDelta) * 100 / float64(totalDelta))
}

func readLoadAverage() (*float64, *float64, *float64) {
	fields := strings.Fields(readTrimmedFile("/proc/loadavg"))
	if len(fields) < 3 {
		return nil, nil, nil
	}
	values := make([]*float64, 3)
	for index := range values {
		value, err := strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return nil, nil, nil
		}
		values[index] = &value
	}
	return values[0], values[1], values[2]
}

func readMemoryInfo() ServerMemoryInfo {
	values := parseMemInfo(readTrimmedFile("/proc/meminfo"))
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if available > total {
		available = total
	}
	used := total - available
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	if swapFree > swapTotal {
		swapFree = swapTotal
	}
	return ServerMemoryInfo{
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: available,
		FreeBytes:      values["MemFree"],
		CachedBytes:    values["Cached"] + values["SReclaimable"],
		BuffersBytes:   values["Buffers"],
		UsagePercent:   percent(used, total),
		SwapTotalBytes: swapTotal,
		SwapUsedBytes:  swapTotal - swapFree,
		SwapFreeBytes:  swapFree,
	}
}

func parseMemInfo(content string) map[string]uint64 {
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		// Linux meminfo values are kB unless explicitly documented otherwise.
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	return values
}

type mountTarget struct {
	device     string
	mountPath  string
	filesystem string
}

func readStorageInfo() ServerStorageInfo {
	targets := readMountTargets()
	if len(targets) == 0 {
		targets = []mountTarget{{mountPath: "/"}}
	}

	storage := ServerStorageInfo{Mounts: make([]ServerStorageMount, 0, len(targets))}
	for _, target := range targets {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(target.mountPath, &stat); err != nil {
			continue
		}
		blockSize := uint64(stat.Bsize)
		total := stat.Blocks * blockSize
		available := stat.Bavail * blockSize
		free := stat.Bfree * blockSize
		used := uint64(0)
		if total > free {
			used = total - free
		}
		storage.Mounts = append(storage.Mounts, ServerStorageMount{
			Device:         target.device,
			MountPath:      target.mountPath,
			Filesystem:     target.filesystem,
			TotalBytes:     total,
			UsedBytes:      used,
			AvailableBytes: available,
			UsagePercent:   percent(used, total),
		})
		storage.TotalBytes += total
		storage.UsedBytes += used
		storage.AvailableBytes += available
	}
	storage.UsagePercent = percent(storage.UsedBytes, storage.TotalBytes)
	return storage
}

func readMountTargets() []mountTarget {
	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil
	}
	defer file.Close()

	seenDevices := make(map[string]bool)
	var targets []mountTarget
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || pseudoFilesystem(fields[2]) {
			continue
		}
		device := unescapeMountField(fields[0])
		mountPath := unescapeMountField(fields[1])
		// Container overlay mounts all report the capacity of their backing
		// filesystem. Count a root overlay once, but do not multiply it by every
		// nested Docker/container mount.
		if device == "overlay" && mountPath != "/" {
			continue
		}
		key := device
		if key == "none" || key == "" {
			key = device + "\x00" + mountPath
		}
		if seenDevices[key] {
			continue
		}
		seenDevices[key] = true
		targets = append(targets, mountTarget{device: device, mountPath: mountPath, filesystem: fields[2]})
	}
	sort.Slice(targets, func(i, j int) bool {
		iRoot := targets[i].mountPath == "/"
		jRoot := targets[j].mountPath == "/"
		if iRoot != jRoot {
			return iRoot
		}
		return targets[i].mountPath < targets[j].mountPath
	})
	return targets
}

func pseudoFilesystem(filesystem string) bool {
	switch filesystem {
	case "autofs", "bpf", "cgroup", "cgroup2", "configfs", "debugfs", "devpts",
		"devtmpfs", "efivarfs", "fusectl", "hugetlbfs", "mqueue", "proc", "pstore",
		"ramfs", "securityfs", "squashfs", "sysfs", "tmpfs", "tracefs":
		return true
	default:
		return false
	}
}

func unescapeMountField(value string) string {
	replacer := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	return replacer.Replace(value)
}

func readNetworkInfo() ServerNetworkInfo {
	counters := readNetworkCounters(readTrimmedFile("/proc/net/dev"))
	interfaces, _ := net.Interfaces()
	result := ServerNetworkInfo{Interfaces: make([]ServerNetworkInterface, 0, len(interfaces))}
	for _, item := range interfaces {
		addresses, _ := item.Addrs()
		addressStrings := make([]string, 0, len(addresses))
		for _, address := range addresses {
			addressStrings = append(addressStrings, address.String())
		}
		counter := counters[item.Name]
		loopback := item.Flags&net.FlagLoopback != 0
		result.Interfaces = append(result.Interfaces, ServerNetworkInterface{
			Name:          item.Name,
			MTU:           item.MTU,
			HardwareAddr:  item.HardwareAddr.String(),
			Addresses:     addressStrings,
			ReceivedBytes: counter[0],
			SentBytes:     counter[1],
			Loopback:      loopback,
			Up:            item.Flags&net.FlagUp != 0,
		})
		if !loopback {
			result.ReceivedBytes += counter[0]
			result.SentBytes += counter[1]
		}
	}
	sort.Slice(result.Interfaces, func(i, j int) bool {
		return result.Interfaces[i].Name < result.Interfaces[j].Name
	})
	return result
}

func readNetworkCounters(content string) map[string][2]uint64 {
	counters := make(map[string][2]uint64)
	for _, line := range strings.Split(content, "\n") {
		nameAndValues := strings.SplitN(line, ":", 2)
		if len(nameAndValues) != 2 {
			continue
		}
		fields := strings.Fields(nameAndValues[1])
		if len(fields) < 9 {
			continue
		}
		received, receiveErr := strconv.ParseUint(fields[0], 10, 64)
		sent, sendErr := strconv.ParseUint(fields[8], 10, 64)
		if receiveErr == nil && sendErr == nil {
			counters[strings.TrimSpace(nameAndValues[0])] = [2]uint64{received, sent}
		}
	}
	return counters
}

func readProcessInfo() ServerProcessInfo {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	openHandles := 0
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		openHandles = len(entries)
	}
	return ServerProcessInfo{
		PID:               os.Getpid(),
		Goroutines:        runtime.NumGoroutine(),
		OpenFileHandles:   openHandles,
		AllocatedBytes:    memory.Alloc,
		HeapInUseBytes:    memory.HeapInuse,
		SystemMemoryBytes: memory.Sys,
	}
}

func readUptime() int64 {
	fields := strings.Fields(readTrimmedFile("/proc/uptime"))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(value)
}

func readOSPrettyName() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
	}
	return ""
}

func readCPUModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "model name") && !strings.HasPrefix(line, "Hardware") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func readTrimmedFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return value[:index]
	}
	return value
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return roundedPercent(float64(used) * 100 / float64(total))
}

func roundedPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10) / 10
}
