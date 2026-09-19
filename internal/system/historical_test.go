package system

import "testing"

func TestRootMajorMinorSelectsOnlyRootBackingDevice(t *testing.T) {
	data := `24 18 8:2 / / rw,relatime - ext4 /dev/sda2 rw
25 24 0:25 / /proc rw,nosuid - proc proc rw
26 24 8:1 / /boot rw,relatime - ext4 /dev/sda1 rw
`
	got, err := rootMajorMinor(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "8:2" {
		t.Fatalf("root major:minor = %q", got)
	}
}

func TestRootMajorMinorRejectsVirtualRoot(t *testing.T) {
	data := `24 18 0:25 / / rw,relatime - overlay overlay rw
`
	if _, err := rootMajorMinor(data); err == nil {
		t.Fatal("expected virtual root to have no block device")
	}
}

func TestParseBlockStatUsesReadAndWriteSectorFields(t *testing.T) {
	read, written, err := parseBlockStat("10 0 123 1 20 0 456 2 0 0 0")
	if err != nil {
		t.Fatal(err)
	}
	if read != 123 || written != 456 {
		t.Fatalf("read = %d, written = %d", read, written)
	}
}
