package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MemoryCollector collects memory usage metrics by reading /proc/meminfo.
type MemoryCollector struct {
	interval    time.Duration
	procMeminfo string
}

// NewMemoryCollector creates a new memory collector with default settings.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{
		interval:    5 * time.Second,
		procMeminfo: "/proc/meminfo",
	}
}

func (m *MemoryCollector) Name() string           { return "memory" }
func (m *MemoryCollector) Interval() time.Duration { return m.interval }

func (m *MemoryCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	info, err := m.readMeminfo()
	if err != nil {
		return nil, fmt.Errorf("memory collect: %w", err)
	}

	total := info["MemTotal"]
	free := info["MemFree"]
	available := info["MemAvailable"]
	buffers := info["Buffers"]
	cached := info["Cached"]
	swapTotal := info["SwapTotal"]
	swapFree := info["SwapFree"]

	used := total - free - buffers - cached
	if used < 0 {
		used = total - free
	}

	var usagePercent float64
	if total > 0 {
		usagePercent = 100.0 * float64(used) / float64(total)
	}

	var swapUsed int64
	var swapPercent float64
	if swapTotal > 0 {
		swapUsed = swapTotal - swapFree
		swapPercent = 100.0 * float64(swapUsed) / float64(swapTotal)
	}

	return []*Metrics{{
		Category:  "memory",
		Timestamp: time.Now(),
		Values: map[string]float64{
			"total":        float64(total),
			"used":         float64(used),
			"free":         float64(free),
			"available":    float64(available),
			"buffers":      float64(buffers),
			"cached":       float64(cached),
			"usage_percent": usagePercent,
			"swap_total":   float64(swapTotal),
			"swap_used":    float64(swapUsed),
			"swap_percent": swapPercent,
		},
	}}, nil
}

func (m *MemoryCollector) readMeminfo() (map[string]int64, error) {
	f, err := os.Open(m.procMeminfo)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := make(map[string]int64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		valStr = strings.TrimSpace(valStr)

		val, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			continue
		}

		// Convert kB to bytes
		info[key] = val * 1024
	}

	return info, scanner.Err()
}
