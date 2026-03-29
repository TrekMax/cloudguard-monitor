package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cpuTimes holds raw CPU time values from /proc/stat.
type cpuTimes struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

func (c cpuTimes) total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
}

func (c cpuTimes) idle() uint64 {
	return c.Idle + c.IOWait
}

// CPUCollector collects CPU usage metrics by reading /proc/stat.
type CPUCollector struct {
	interval time.Duration
	mu       sync.Mutex
	prev     *cpuTimes
	procStat string // path to proc/stat, for testing
}

// NewCPUCollector creates a new CPU collector with default settings.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{
		interval: 5 * time.Second,
		procStat: "/proc/stat",
	}
}

func (c *CPUCollector) Name() string            { return "cpu" }
func (c *CPUCollector) Interval() time.Duration { return c.interval }

func (c *CPUCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	current, perCore, err := c.readProcStat()
	if err != nil {
		return nil, fmt.Errorf("cpu collect: %w", err)
	}

	now := time.Now()

	c.mu.Lock()
	prev := c.prev
	c.prev = current
	c.mu.Unlock()

	// First collection: no previous data, return 0
	if prev == nil {
		return []*Metrics{{
			Category:  "cpu",
			Timestamp: now,
			Values: map[string]float64{
				"usage":  0,
				"user":   0,
				"system": 0,
				"idle":   100,
				"iowait": 0,
				"cores":  float64(len(perCore)),
			},
		}}, nil
	}

	totalDelta := current.total() - prev.total()
	if totalDelta == 0 {
		return []*Metrics{{
			Category:  "cpu",
			Timestamp: now,
			Values: map[string]float64{
				"usage":  0,
				"user":   0,
				"system": 0,
				"idle":   100,
				"iowait": 0,
				"cores":  float64(len(perCore)),
			},
		}}, nil
	}

	usage := 100.0 * float64(totalDelta-current.idle()+prev.idle()) / float64(totalDelta)
	user := 100.0 * float64(current.User-prev.User) / float64(totalDelta)
	system := 100.0 * float64(current.System-prev.System) / float64(totalDelta)
	idlePct := 100.0 * float64(current.idle()-prev.idle()) / float64(totalDelta)
	iowait := 100.0 * float64(current.IOWait-prev.IOWait) / float64(totalDelta)

	// Read load averages
	load1, load5, load15 := readLoadAvg()

	return []*Metrics{{
		Category:  "cpu",
		Timestamp: now,
		Values: map[string]float64{
			"usage":  usage,
			"user":   user,
			"system": system,
			"idle":   idlePct,
			"iowait": iowait,
			"load1":  load1,
			"load5":  load5,
			"load15": load15,
			"cores":  float64(len(perCore)),
		},
	}}, nil
}

func (c *CPUCollector) readProcStat() (*cpuTimes, []cpuTimes, error) {
	f, err := os.Open(c.procStat)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var total *cpuTimes
	var perCore []cpuTimes

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			t, err := parseCPULine(line)
			if err != nil {
				return nil, nil, err
			}
			total = t
		} else if strings.HasPrefix(line, "cpu") {
			t, err := parseCPULine(line)
			if err != nil {
				continue
			}
			perCore = append(perCore, *t)
		}
	}

	if total == nil {
		return nil, nil, fmt.Errorf("no cpu line found in %s", c.procStat)
	}

	return total, perCore, scanner.Err()
}

func parseCPULine(line string) (*cpuTimes, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil, fmt.Errorf("unexpected cpu line format: %s", line)
	}

	values := make([]uint64, 8)
	for i := 0; i < 8 && i+1 < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i+1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cpu field %d: %w", i, err)
		}
		values[i] = v
	}

	return &cpuTimes{
		User:    values[0],
		Nice:    values[1],
		System:  values[2],
		Idle:    values[3],
		IOWait:  values[4],
		IRQ:     values[5],
		SoftIRQ: values[6],
		Steal:   values[7],
	}, nil
}

func readLoadAvg() (load1, load5, load15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}

	load1, _ = strconv.ParseFloat(fields[0], 64)
	load5, _ = strconv.ParseFloat(fields[1], 64)
	load15, _ = strconv.ParseFloat(fields[2], 64)
	return
}
