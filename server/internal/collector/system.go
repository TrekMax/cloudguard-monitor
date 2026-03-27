package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemInfo holds static system information collected once at startup.
type SystemInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Platform    string `json:"platform"`
	Kernel      string `json:"kernel"`
	Arch        string `json:"arch"`
	CPUCores    int    `json:"cpu_cores"`
	BootTime    int64  `json:"boot_time"`
	Uptime      int64  `json:"uptime"` // seconds
	GoVersion   string `json:"go_version"`
	AgentVersion string `json:"agent_version"`
}

// CollectSystemInfo gathers static system information.
func CollectSystemInfo() *SystemInfo {
	info := &SystemInfo{
		Arch:         runtime.GOARCH,
		CPUCores:     runtime.NumCPU(),
		GoVersion:    runtime.Version(),
		AgentVersion: "0.1.0",
	}

	info.Hostname, _ = os.Hostname()
	info.Kernel = readFileFirstLine("/proc/version")
	info.OS = readOSName()
	info.Platform = runtime.GOOS

	// Boot time from /proc/stat
	info.BootTime = readBootTime()
	if info.BootTime > 0 {
		info.Uptime = time.Now().Unix() - info.BootTime
	}

	return info
}

func readFileFirstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func readOSName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name := strings.TrimPrefix(line, "PRETTY_NAME=")
			name = strings.Trim(name, "\"")
			return name
		}
	}
	return runtime.GOOS
}

func readBootTime() int64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "btime ") {
			val, err := strconv.ParseInt(strings.TrimPrefix(line, "btime "), 10, 64)
			if err != nil {
				return 0
			}
			return val
		}
	}
	return 0
}

// ProcessInfo holds information about a single process.
type ProcessInfo struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	State   string  `json:"state"`
	CPUPct  float64 `json:"cpu_percent"`
	MemRSS int64   `json:"mem_rss"` // bytes
	User    string  `json:"user"`
}

// ProcessCollector collects top N processes by CPU/memory usage.
type ProcessCollector struct {
	interval time.Duration
	topN     int
	procPath string
}

func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		interval: 10 * time.Second,
		topN:     10,
		procPath: "/proc",
	}
}

func (p *ProcessCollector) Name() string           { return "process" }
func (p *ProcessCollector) Interval() time.Duration { return p.interval }

func (p *ProcessCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	procs, err := p.readProcesses()
	if err != nil {
		return nil, fmt.Errorf("process collect: %w", err)
	}

	now := time.Now()

	// Total process count
	totalProcs := len(procs)

	// Sort by RSS (memory) to get top N
	sortByRSS(procs)
	topMem := procs
	if len(topMem) > p.topN {
		topMem = topMem[:p.topN]
	}

	results := []*Metrics{{
		Category:  "process",
		Timestamp: now,
		Values: map[string]float64{
			"total": float64(totalProcs),
		},
	}}

	for _, proc := range topMem {
		results = append(results, &Metrics{
			Category:  "process",
			Timestamp: now,
			Values: map[string]float64{
				"pid":     float64(proc.PID),
				"mem_rss": float64(proc.MemRSS),
			},
			Labels: map[string]string{
				"name":  proc.Name,
				"state": proc.State,
			},
		})
	}

	return results, nil
}

func (p *ProcessCollector) readProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir(p.procPath)
	if err != nil {
		return nil, err
	}

	var procs []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		proc := p.readOneProcess(pid)
		if proc.Name != "" {
			procs = append(procs, proc)
		}
	}

	return procs, nil
}

func (p *ProcessCollector) readOneProcess(pid int) ProcessInfo {
	statPath := fmt.Sprintf("%s/%d/stat", p.procPath, pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return ProcessInfo{}
	}

	content := string(data)

	// Parse comm (name) — it's between parentheses and may contain spaces
	openParen := strings.IndexByte(content, '(')
	closeParen := strings.LastIndexByte(content, ')')
	if openParen < 0 || closeParen < 0 || closeParen <= openParen {
		return ProcessInfo{}
	}

	name := content[openParen+1 : closeParen]
	rest := strings.Fields(content[closeParen+2:])
	if len(rest) < 22 {
		return ProcessInfo{}
	}

	state := rest[0]
	rssPages, _ := strconv.ParseInt(rest[21], 10, 64)
	pageSize := int64(os.Getpagesize())

	return ProcessInfo{
		PID:    pid,
		Name:   name,
		State:  state,
		MemRSS: rssPages * pageSize,
	}
}

func sortByRSS(procs []ProcessInfo) {
	// Simple insertion sort — good enough for process lists
	for i := 1; i < len(procs); i++ {
		key := procs[i]
		j := i - 1
		for j >= 0 && procs[j].MemRSS < key.MemRSS {
			procs[j+1] = procs[j]
			j--
		}
		procs[j+1] = key
	}
}
