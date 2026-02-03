// Package sysmetrics collects basic system metrics (CPU, memory) for heartbeat reporting.
// This replaces the Python psutil dependency with pure Go + OS commands.
// Falls back gracefully if metrics cannot be collected (returns nil).
package sysmetrics

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Metrics holds system resource information sent with heartbeats.
type Metrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	ActiveTests   int     `json:"active_tests"`
}

// Collect gathers CPU and memory usage. Returns nil if metrics are unavailable.
func Collect(activeTests int) *Metrics {
	cpu := getCPUPercent()
	mem := getMemoryPercent()

	if cpu < 0 && mem < 0 {
		return nil
	}

	if cpu < 0 {
		cpu = 0
	}
	if mem < 0 {
		mem = 0
	}

	return &Metrics{
		CPUPercent:    cpu,
		MemoryPercent: mem,
		ActiveTests:   activeTests,
	}
}

func getCPUPercent() float64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sh", "-c", "top -l 1 -n 0 | grep 'CPU usage'").Output()
		if err != nil {
			return -1
		}
		line := string(out)
		if idx := strings.Index(line, "idle"); idx > 0 {
			parts := strings.Split(line[:idx], ",")
			if len(parts) >= 1 {
				last := strings.TrimSpace(parts[len(parts)-1])
				last = strings.TrimSuffix(last, "%")
				last = strings.TrimSpace(last)
				idle, err := strconv.ParseFloat(last, 64)
				if err == nil {
					return 100 - idle
				}
			}
		}
	case "linux":
		out, err := exec.Command("sh", "-c", "grep 'cpu ' /proc/stat").Output()
		if err != nil {
			return -1
		}
		fields := strings.Fields(string(out))
		if len(fields) >= 5 {
			user, _ := strconv.ParseFloat(fields[1], 64)
			nice, _ := strconv.ParseFloat(fields[2], 64)
			system, _ := strconv.ParseFloat(fields[3], 64)
			idle, _ := strconv.ParseFloat(fields[4], 64)
			total := user + nice + system + idle
			if total > 0 {
				return ((total - idle) / total) * 100
			}
		}
	}
	return -1
}

func getMemoryPercent() float64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("vm_stat").Output()
		if err != nil {
			return -1
		}
		lines := strings.Split(string(out), "\n")
		pageSize := 16384.0
		if len(lines) > 0 && strings.Contains(lines[0], "page size of") {
			parts := strings.Fields(lines[0])
			for i, p := range parts {
				if p == "of" && i+1 < len(parts) {
					ps, err := strconv.ParseFloat(strings.TrimRight(parts[i+1], "."), 64)
					if err == nil {
						pageSize = ps
					}
				}
			}
		}

		var free, active, inactive, speculative, wired float64
		for _, line := range lines {
			if v, ok := parseVMStatLine(line, "Pages free"); ok {
				free = v
			} else if v, ok := parseVMStatLine(line, "Pages active"); ok {
				active = v
			} else if v, ok := parseVMStatLine(line, "Pages inactive"); ok {
				inactive = v
			} else if v, ok := parseVMStatLine(line, "Pages speculative"); ok {
				speculative = v
			} else if v, ok := parseVMStatLine(line, "Pages wired down"); ok {
				wired = v
			}
		}

		total := (free + active + inactive + speculative + wired) * pageSize
		used := (active + wired) * pageSize
		if total > 0 {
			return (used / total) * 100
		}
	case "linux":
		out, err := exec.Command("sh", "-c", "grep -E '^(MemTotal|MemAvailable):' /proc/meminfo").Output()
		if err != nil {
			return -1
		}
		lines := strings.Split(string(out), "\n")
		var total, available float64
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseFloat(fields[1], 64)
				if strings.HasPrefix(line, "MemTotal") {
					total = val
				} else if strings.HasPrefix(line, "MemAvailable") {
					available = val
				}
			}
		}
		if total > 0 {
			return ((total - available) / total) * 100
		}
	}
	return -1
}

func parseVMStatLine(line, prefix string) (float64, bool) {
	if strings.Contains(line, prefix) {
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			val = strings.TrimSuffix(val, ".")
			v, err := strconv.ParseFloat(val, 64)
			if err == nil {
				return v, true
			}
		}
	}
	return 0, false
}
