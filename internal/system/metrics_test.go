package system

import "testing"

func TestCalculateCPUUsage(t *testing.T) {
	first := cpuTimes{idle: 400, total: 1000}
	second := cpuTimes{idle: 450, total: 1200}

	if got := calculateCPUUsage(first, second); got != 75 {
		t.Fatalf("usage = %v, want 75", got)
	}
}

func TestParseMeminfo(t *testing.T) {
	data := `MemTotal:       1000 kB
MemAvailable:    250 kB
SwapTotal:       500 kB
SwapFree:        400 kB
`

	got, err := parseMeminfo(data)
	if err != nil {
		t.Fatal(err)
	}

	if got.TotalBytes != 1000*1024 || got.UsedBytes != 750*1024 {
		t.Fatalf("unexpected memory totals: %+v", got)
	}
	if got.UsagePercent != 75 || got.SwapUsagePercent != 20 {
		t.Fatalf("unexpected usage percentages: %+v", got)
	}
}

func TestParseLoadAverage(t *testing.T) {
	one, five, fifteen, err := parseLoadAverage("0.12 0.08 0.09 1/799 123\n")
	if err != nil {
		t.Fatal(err)
	}
	if one != 0.12 || five != 0.08 || fifteen != 0.09 {
		t.Fatalf("loads = %v %v %v", one, five, fifteen)
	}
}

func TestParseDefaultInterface(t *testing.T) {
	data := `Iface	Destination	Gateway	Flags	RefCnt	Use	Metric	Mask
wlp2s0	00000000	0101A8C0	0003	0	0	600	00000000
docker0	000011AC	00000000	0001	0	0	0	0000FFFF
`

	got, err := parseDefaultInterface(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "wlp2s0" {
		t.Fatalf("interface = %q, want wlp2s0", got)
	}
}
