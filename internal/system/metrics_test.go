package system

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestReadPowerSupply(t *testing.T) {
	tests := []struct {
		name        string
		devices     map[string]map[string]string
		available   bool
		acAvailable bool
		percent     float64
		connected   bool
		status      string
	}{
		{
			name: "battery detected with AC connected",
			devices: map[string]map[string]string{
				"BAT1": {
					"type": "Battery", "capacity": "87", "status": "Charging",
				},
				"ACAD": {"type": "Mains", "online": "1"},
			},
			available:   true,
			acAvailable: true,
			percent:     87,
			connected:   true,
			status:      "Charging",
		},
		{
			name: "AC disconnected",
			devices: map[string]map[string]string{
				"CMB0": {
					"type": "Battery", "capacity": "64", "status": "Discharging",
				},
				"ADP0": {"type": "AC", "online": "0"},
			},
			available:   true,
			acAvailable: true,
			percent:     64,
			connected:   false,
			status:      "Discharging",
		},
		{
			name: "battery marked absent while mains remains connected",
			devices: map[string]map[string]string{
				"AC": {"type": "Mains", "online": "1"},
				"BAT1": {
					"type": "Battery", "present": "0", "capacity": "91",
					"status": "Charging",
				},
			},
			acAvailable: true,
			connected:   true,
		},
		{
			name: "malformed optional devices are ignored",
			devices: map[string]map[string]string{
				"BROKEN": {"type": "Battery", "capacity": "not-a-number"},
				"USB":    {"type": "USB", "online": "1"},
			},
		},
		{
			name: "battery status safely infers AC without a mains device",
			devices: map[string]map[string]string{
				"dell-battery": {
					"type": "Battery", "capacity": "100", "status": "Full",
				},
			},
			available: true,
			percent:   100,
			connected: true,
			status:    "Full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for device, files := range test.devices {
				deviceRoot := filepath.Join(root, device)
				if err := os.Mkdir(deviceRoot, 0o755); err != nil {
					t.Fatal(err)
				}
				for name, content := range files {
					if err := os.WriteFile(
						filepath.Join(deviceRoot, name),
						[]byte(content),
						0o644,
					); err != nil {
						t.Fatal(err)
					}
				}
			}

			got := readPowerSupply(root)
			if got.Available != test.available ||
				got.ACAvailable != test.acAvailable ||
				got.Percent != test.percent ||
				got.ACConnected != test.connected ||
				got.Status != test.status {
				t.Fatalf("battery = %+v", got)
			}
		})
	}
}

func TestReadPowerSupplyMissingDirectory(t *testing.T) {
	got := readPowerSupply(filepath.Join(t.TempDir(), "missing"))
	if got.Available || got.ACAvailable || got.Percent != 0 ||
		got.ACConnected || got.Status != "" {
		t.Fatalf("battery = %+v, want unavailable", got)
	}
}
