package system

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const sampleInterval = 200 * time.Millisecond

type CPUStats struct {
	UsagePercent float64 `json:"usagePercent"`
	LogicalCores int     `json:"logicalCores"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type MemoryStats struct {
	TotalBytes       uint64  `json:"totalBytes"`
	UsedBytes        uint64  `json:"usedBytes"`
	AvailableBytes   uint64  `json:"availableBytes"`
	UsagePercent     float64 `json:"usagePercent"`
	SwapTotalBytes   uint64  `json:"swapTotalBytes"`
	SwapUsedBytes    uint64  `json:"swapUsedBytes"`
	SwapUsagePercent float64 `json:"swapUsagePercent"`
}

type DiskStats struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type TemperatureStats struct {
	Celsius float64 `json:"celsius"`
	Source  string  `json:"source"`
}

type NetworkStats struct {
	Interface     string  `json:"interface"`
	RXBytes       uint64  `json:"rxBytes"`
	TXBytes       uint64  `json:"txBytes"`
	RXBytesPerSec float64 `json:"rxBytesPerSecond"`
	TXBytesPerSec float64 `json:"txBytesPerSecond"`
}

type ServiceStatus struct {
	Name   string `json:"name"`
	Unit   string `json:"unit"`
	Status string `json:"status"`
	Active bool   `json:"active"`
}

type Metrics struct {
	CPU           CPUStats         `json:"cpu"`
	Memory        MemoryStats      `json:"memory"`
	Disk          DiskStats        `json:"disk"`
	Temperature   TemperatureStats `json:"temperature"`
	UptimeSeconds float64          `json:"uptimeSeconds"`
	Network       NetworkStats     `json:"network"`
	Services      []ServiceStatus  `json:"services"`
	CollectedAt   time.Time        `json:"collectedAt"`
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

type networkCounters struct {
	rx uint64
	tx uint64
}

func Collect() (Metrics, error) {
	firstCPU, err := readCPUStat()
	if err != nil {
		return Metrics{}, err
	}

	iface, err := defaultInterface()
	if err != nil {
		return Metrics{}, err
	}

	firstNetwork, err := readNetworkCounters(iface)
	if err != nil {
		return Metrics{}, err
	}

	time.Sleep(sampleInterval)

	secondCPU, err := readCPUStat()
	if err != nil {
		return Metrics{}, err
	}

	secondNetwork, err := readNetworkCounters(iface)
	if err != nil {
		return Metrics{}, err
	}

	load1, load5, load15, err := readLoadAverage()
	if err != nil {
		return Metrics{}, err
	}

	memory, err := readMemory()
	if err != nil {
		return Metrics{}, err
	}

	disk, err := readDisk("/")
	if err != nil {
		return Metrics{}, err
	}

	uptime, err := readUptime()
	if err != nil {
		return Metrics{}, err
	}

	seconds := sampleInterval.Seconds()

	return Metrics{
		CPU: CPUStats{
			UsagePercent: calculateCPUUsage(firstCPU, secondCPU),
			LogicalCores: runtime.NumCPU(),
			Load1:        load1,
			Load5:        load5,
			Load15:       load15,
		},
		Memory:        memory,
		Disk:          disk,
		Temperature:   readCPUTemperature(),
		UptimeSeconds: uptime,
		Network: NetworkStats{
			Interface:     iface,
			RXBytes:       secondNetwork.rx,
			TXBytes:       secondNetwork.tx,
			RXBytesPerSec: counterRate(firstNetwork.rx, secondNetwork.rx, seconds),
			TXBytesPerSec: counterRate(firstNetwork.tx, secondNetwork.tx, seconds),
		},
		Services:    readServices(),
		CollectedAt: time.Now().UTC(),
	}, nil
}

func readCPUStat() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	return parseCPUStat(string(data))
}

func parseCPUStat(data string) (cpuTimes, error) {
	line, _, _ := strings.Cut(data, "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, errors.New("invalid /proc/stat cpu line")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parse cpu field: %w", err)
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
	return cpuTimes{idle: idle, total: total}, nil
}

func calculateCPUUsage(first, second cpuTimes) float64 {
	if second.total <= first.total || second.idle < first.idle {
		return 0
	}

	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if idleDelta > totalDelta {
		return 0
	}

	return clampPercent(float64(totalDelta-idleDelta) / float64(totalDelta) * 100)
}

func readLoadAverage() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	return parseLoadAverage(string(data))
}

func parseLoadAverage(data string) (float64, float64, float64, error) {
	fields := strings.Fields(data)
	if len(fields) < 3 {
		return 0, 0, 0, errors.New("invalid /proc/loadavg")
	}

	values := make([]float64, 3)
	for i := range values {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, 0, 0, err
		}
		values[i] = value
	}
	return values[0], values[1], values[2], nil
}

func readMemory() (MemoryStats, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryStats{}, err
	}
	return parseMeminfo(string(data))
}

func parseMeminfo(data string) (MemoryStats, error) {
	values := map[string]uint64{}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return MemoryStats{}, err
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return MemoryStats{}, errors.New("MemTotal missing from /proc/meminfo")
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
	swapUsed := swapTotal - swapFree

	return MemoryStats{
		TotalBytes:       total,
		UsedBytes:        used,
		AvailableBytes:   available,
		UsagePercent:     percent(used, total),
		SwapTotalBytes:   swapTotal,
		SwapUsedBytes:    swapUsed,
		SwapUsagePercent: percent(swapUsed, swapTotal),
	}, nil
}

func readDisk(mount string) (DiskStats, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(mount, &stats); err != nil {
		return DiskStats{}, err
	}

	total := stats.Blocks * uint64(stats.Bsize)
	available := stats.Bavail * uint64(stats.Bsize)
	free := stats.Bfree * uint64(stats.Bsize)
	used := total - free

	return DiskStats{
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: available,
		UsagePercent:   percent(used, used+available),
	}, nil
}

func readUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("invalid /proc/uptime")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func defaultInterface() (string, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", err
	}
	return parseDefaultInterface(string(data))
}

func parseDefaultInterface(data string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == "00000000" && fields[0] != "Iface" {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("default network interface not found")
}

func readNetworkCounters(iface string) (networkCounters, error) {
	rx, err := readUintFile(filepath.Join("/sys/class/net", iface, "statistics/rx_bytes"))
	if err != nil {
		return networkCounters{}, err
	}
	tx, err := readUintFile(filepath.Join("/sys/class/net", iface, "statistics/tx_bytes"))
	if err != nil {
		return networkCounters{}, err
	}
	return networkCounters{rx: rx, tx: tx}, nil
}

func readCPUTemperature() TemperatureStats {
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hwmon := range hwmons {
		nameData, err := os.ReadFile(filepath.Join(hwmon, "name"))
		if err != nil || strings.TrimSpace(string(nameData)) != "k10temp" {
			continue
		}
		value, err := readUintFile(filepath.Join(hwmon, "temp1_input"))
		if err == nil {
			return TemperatureStats{Celsius: float64(value) / 1000, Source: "k10temp"}
		}
	}

	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, zone := range zones {
		value, err := readUintFile(filepath.Join(zone, "temp"))
		if err != nil {
			continue
		}
		typeData, _ := os.ReadFile(filepath.Join(zone, "type"))
		return TemperatureStats{
			Celsius: float64(value) / 1000,
			Source:  strings.TrimSpace(string(typeData)),
		}
	}
	return TemperatureStats{}
}

func readServices() []ServiceStatus {
	definitions := []struct {
		name string
		unit string
	}{
		{"Docker", "docker.service"},
		{"MiniDeploy", "minideploy.service"},
		{"MiniBase", "minibase.service"},
		{"Caddy", "caddy.service"},
		{"cloudflared", "cloudflared.service"},
		{"ReactorLab", "reactorlab.service"},
		{"MiniBase backups", "minibase-backup.timer"},
	}

	args := []string{"is-active"}
	for _, definition := range definitions {
		args = append(args, definition.unit)
	}
	output, _ := exec.Command("systemctl", args...).CombinedOutput()
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	result := make([]ServiceStatus, 0, len(definitions))
	for i, definition := range definitions {
		status := "unknown"
		if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			status = strings.TrimSpace(lines[i])
		}
		result = append(result, ServiceStatus{
			Name:   definition.name,
			Unit:   definition.unit,
			Status: status,
			Active: status == "active",
		})
	}
	return result
}

func readUintFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func counterRate(first, second uint64, seconds float64) float64 {
	if second < first || seconds <= 0 {
		return 0
	}
	return float64(second-first) / seconds
}

func percent(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return clampPercent(float64(part) / float64(total) * 100)
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
