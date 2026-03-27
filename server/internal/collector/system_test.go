package collector

import (
	"context"
	"runtime"
	"testing"
)

func TestCollectSystemInfo(t *testing.T) {
	info := CollectSystemInfo()

	if info.Hostname == "" {
		t.Error("hostname should not be empty")
	}
	if info.CPUCores <= 0 {
		t.Error("cpu cores should be positive")
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.Platform != runtime.GOOS {
		t.Errorf("platform = %q, want %q", info.Platform, runtime.GOOS)
	}
	if info.GoVersion == "" {
		t.Error("go version should not be empty")
	}
	if info.AgentVersion != "0.1.0" {
		t.Errorf("agent version = %q, want '0.1.0'", info.AgentVersion)
	}
}

func TestProcessCollector_Name(t *testing.T) {
	p := NewProcessCollector()
	if p.Name() != "process" {
		t.Errorf("expected name 'process', got %q", p.Name())
	}
}

func TestProcessCollector_Collect(t *testing.T) {
	p := NewProcessCollector()

	metrics, err := p.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	// First metric should have total process count
	total := metrics[0].Values["total"]
	if total <= 0 {
		t.Errorf("total processes should be positive, got %f", total)
	}
}

func TestSortByRSS(t *testing.T) {
	procs := []ProcessInfo{
		{PID: 1, MemRSS: 100},
		{PID: 2, MemRSS: 500},
		{PID: 3, MemRSS: 200},
	}
	sortByRSS(procs)

	if procs[0].PID != 2 || procs[1].PID != 3 || procs[2].PID != 1 {
		t.Errorf("sort order wrong: %v", procs)
	}
}
