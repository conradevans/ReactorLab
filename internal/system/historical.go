package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type HistoricalSnapshot struct {
	ObservedAt       time.Time
	CPUIdle          uint64
	CPUTotal         uint64
	MemoryUsedBytes  uint64
	MemoryTotalBytes uint64
	Load1            float64
	Load5            float64
	Load15           float64
	DiskUsedBytes    uint64
	DiskTotalBytes   uint64
	DiskReadBytes    *uint64
	DiskWriteBytes   *uint64
	NetworkInterface string
	NetworkRXBytes   uint64
	NetworkTXBytes   uint64
	UptimeSeconds    float64
	BootID           string
}

func ReadHistoricalSnapshot(now time.Time) (HistoricalSnapshot, error) {
	cpu, err := readCPUStat()
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	memory, err := readMemory()
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	load1, load5, load15, err := readLoadAverage()
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	disk, err := readDisk("/")
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	var iface string
	var network networkCounters
	if candidate, interfaceErr := defaultInterface(); interfaceErr == nil && candidate != "lo" {
		if counters, counterErr := readNetworkCounters(candidate); counterErr == nil {
			iface = candidate
			network = counters
		}
	}
	uptime, err := readUptime()
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	bootData, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	readBytes, writeBytes, _ := readRootDiskIO("/proc/self/mountinfo", "/sys")
	return HistoricalSnapshot{
		ObservedAt: now.UTC(), CPUIdle: cpu.idle, CPUTotal: cpu.total,
		MemoryUsedBytes: memory.UsedBytes, MemoryTotalBytes: memory.TotalBytes,
		Load1: load1, Load5: load5, Load15: load15,
		DiskUsedBytes: disk.UsedBytes, DiskTotalBytes: disk.TotalBytes,
		DiskReadBytes: readBytes, DiskWriteBytes: writeBytes,
		NetworkInterface: iface, NetworkRXBytes: network.rx, NetworkTXBytes: network.tx,
		UptimeSeconds: uptime, BootID: strings.TrimSpace(string(bootData)),
	}, nil
}

func ReadTemperature() (TemperatureStats, bool) {
	temperature := readCPUTemperature()
	return temperature, temperature.Source != ""
}

func readRootDiskIO(mountInfoPath, sysRoot string) (*uint64, *uint64, error) {
	data, err := os.ReadFile(mountInfoPath)
	if err != nil {
		return nil, nil, err
	}
	majorMinor, err := rootMajorMinor(string(data))
	if err != nil {
		return nil, nil, err
	}
	link, err := filepath.EvalSymlinks(filepath.Join(sysRoot, "dev/block", majorMinor))
	if err != nil {
		return nil, nil, err
	}
	device := filepath.Base(link)
	if _, err := os.Stat(filepath.Join(sysRoot, "class/block", device, "partition")); err == nil {
		device = filepath.Base(filepath.Dir(link))
	}
	statData, err := os.ReadFile(filepath.Join(sysRoot, "class/block", device, "stat"))
	if err != nil {
		return nil, nil, err
	}
	readSectors, writeSectors, err := parseBlockStat(string(statData))
	if err != nil {
		return nil, nil, err
	}
	// Linux diskstats sectors are defined as 512 bytes, independent of device block size.
	readBytes := readSectors * 512
	writeBytes := writeSectors * 512
	return &readBytes, &writeBytes, nil
}

func rootMajorMinor(data string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 5 && fields[4] == "/" {
			parts := strings.Split(fields[2], ":")
			if len(parts) != 2 {
				break
			}
			if parts[0] == "0" {
				return "", errors.New("root filesystem has no block device")
			}
			return fields[2], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("root mount not found")
}

func parseBlockStat(data string) (uint64, uint64, error) {
	fields := strings.Fields(data)
	if len(fields) < 7 {
		return 0, 0, errors.New("invalid block device stat")
	}
	readSectors, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse block read sectors: %w", err)
	}
	writeSectors, err := strconv.ParseUint(fields[6], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse block write sectors: %w", err)
	}
	return readSectors, writeSectors, nil
}

func ServiceInvocationIDs(ctx context.Context, units []string) map[string]string {
	result := make(map[string]string)
	if len(units) == 0 {
		return result
	}
	args := append([]string{"show", "--property=Id", "--property=InvocationID"}, units...)
	output, err := exec.CommandContext(ctx, "systemctl", args...).Output()
	if err != nil {
		return result
	}
	var id string
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Id":
			id = strings.TrimSpace(value)
		case "InvocationID":
			if id != "" && strings.TrimSpace(value) != "" {
				result[id] = strings.TrimSpace(value)
			}
			id = ""
		}
	}
	return result
}
