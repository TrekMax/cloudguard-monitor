package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1234567   12345    0    0    0     0          0         0  1234567   12345    0    0    0     0       0          0
  eth0: 98765432  654321    0    0    0     0          0         0 12345678  123456    0    0    0     0       0          0
`

const testNetDev2 = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1234567   12345    0    0    0     0          0         0  1234567   12345    0    0    0     0       0          0
  eth0: 99765432  664321    0    0    0     0          0         0 13345678  133456    0    0    0     0       0          0
`

func TestNetworkCollector_Name(t *testing.T) {
	n := NewNetworkCollector()
	if n.Name() != "network" {
		t.Errorf("expected name 'network', got %q", n.Name())
	}
}

func TestNetworkCollector_FirstCollect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")
	os.WriteFile(path, []byte(testNetDev), 0644)

	n := NewNetworkCollector()
	n.procNetDev = path

	metrics, err := n.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	// First collect should have 0 rate
	found := false
	for _, m := range metrics {
		if m.Labels == nil || len(m.Labels) == 0 {
			// This is the aggregate metric
			found = true
			if m.Values["rx_bytes_rate"] != 0 {
				t.Errorf("first collect rx rate should be 0, got %f", m.Values["rx_bytes_rate"])
			}
			if m.Values["rx_bytes"] != 98765432 {
				t.Errorf("rx_bytes = %f, want 98765432", m.Values["rx_bytes"])
			}
		}
	}
	if !found {
		t.Error("aggregate network metric not found")
	}
}

func TestNetworkCollector_SecondCollect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")
	os.WriteFile(path, []byte(testNetDev), 0644)

	n := NewNetworkCollector()
	n.procNetDev = path

	// First collect
	_, err := n.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Update net dev
	os.WriteFile(path, []byte(testNetDev2), 0644)

	// Second collect
	metrics, err := n.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Should have aggregate + per-interface metrics
	if len(metrics) < 2 {
		t.Errorf("expected at least 2 metrics (aggregate + per-interface), got %d", len(metrics))
	}

	// Aggregate should have positive rate
	for _, m := range metrics {
		if m.Labels == nil || len(m.Labels) == 0 {
			if m.Values["rx_bytes_rate"] <= 0 {
				t.Errorf("second collect should have positive rx rate, got %f", m.Values["rx_bytes_rate"])
			}
		}
	}
}

func TestNetworkCollector_InvalidPath(t *testing.T) {
	n := NewNetworkCollector()
	n.procNetDev = "/nonexistent/net_dev"

	_, err := n.Collect(context.Background())
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
