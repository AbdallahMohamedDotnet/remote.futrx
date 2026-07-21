package hostinfo

import "testing"

func TestParseMemInfo(t *testing.T) {
	values := parseMemInfo("MemTotal:       1000 kB\nMemAvailable:    400 kB\nSwapTotal:       200 kB\n")
	if values["MemTotal"] != 1000*1024 || values["MemAvailable"] != 400*1024 {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestReadNetworkCounters(t *testing.T) {
	values := readNetworkCounters(`Inter-| Receive | Transmit
 eth0: 1024 1 2 3 4 5 6 7 2048 9 10 11 12 13 14 15
    lo:  100 1 2 3 4 5 6 7  100 9 10 11 12 13 14 15`)
	if values["eth0"] != [2]uint64{1024, 2048} {
		t.Fatalf("unexpected counters: %#v", values)
	}
}

func TestCPUUsage(t *testing.T) {
	got := cpuUsage(cpuSample{total: 100, idle: 40}, cpuSample{total: 200, idle: 70})
	if got != 70 {
		t.Fatalf("usage = %v, want 70", got)
	}
}
