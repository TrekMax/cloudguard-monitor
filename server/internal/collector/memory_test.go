package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testMeminfo = `MemTotal:        8167848 kB
MemFree:         1744524 kB
MemAvailable:    5765498 kB
Buffers:          234128 kB
Cached:          3788860 kB
SwapCached:            0 kB
Active:          3250180 kB
Inactive:        2674312 kB
SwapTotal:       2097148 kB
SwapFree:        2097148 kB
`

func TestMemoryCollector_Name(t *testing.T) {
	m := NewMemoryCollector()
	if m.Name() != "memory" {
		t.Errorf("expected name 'memory', got %q", m.Name())
	}
}

func TestMemoryCollector_Collect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meminfo")
	if err := os.WriteFile(path, []byte(testMeminfo), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewMemoryCollector()
	m.procMeminfo = path

	metrics, err := m.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	vals := metrics[0].Values

	// MemTotal = 8167848 kB = 8,363,876,352 bytes
	expectedTotal := float64(8167848 * 1024)
	if vals["total"] != expectedTotal {
		t.Errorf("total = %f, want %f", vals["total"], expectedTotal)
	}

	if vals["usage_percent"] < 0 || vals["usage_percent"] > 100 {
		t.Errorf("usage_percent out of range: %f", vals["usage_percent"])
	}

	// Used = Total - Free - Buffers - Cached
	expectedUsed := float64((8167848 - 1744524 - 234128 - 3788860) * 1024)
	if vals["used"] != expectedUsed {
		t.Errorf("used = %f, want %f", vals["used"], expectedUsed)
	}

	// Swap should be 0% used (SwapFree == SwapTotal)
	if vals["swap_percent"] != 0 {
		t.Errorf("swap_percent = %f, want 0", vals["swap_percent"])
	}

	if metrics[0].Category != "memory" {
		t.Errorf("category = %q, want 'memory'", metrics[0].Category)
	}
}

func TestMemoryCollector_WithSwapUsed(t *testing.T) {
	meminfo := `MemTotal:        8167848 kB
MemFree:         1744524 kB
MemAvailable:    5765498 kB
Buffers:          234128 kB
Cached:          3788860 kB
SwapTotal:       2097148 kB
SwapFree:        1048576 kB
`
	dir := t.TempDir()
	path := filepath.Join(dir, "meminfo")
	if err := os.WriteFile(path, []byte(meminfo), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewMemoryCollector()
	m.procMeminfo = path

	metrics, err := m.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	swapPercent := metrics[0].Values["swap_percent"]
	if swapPercent <= 0 || swapPercent >= 100 {
		t.Errorf("swap_percent should be between 0 and 100, got %f", swapPercent)
	}
}

func TestMemoryCollector_InvalidPath(t *testing.T) {
	m := NewMemoryCollector()
	m.procMeminfo = "/nonexistent/meminfo"

	_, err := m.Collect(context.Background())
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
