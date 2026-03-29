package store

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s, err := New(dbPath, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNew(t *testing.T) {
	s := testStore(t)
	if s == nil {
		t.Fatal("store should not be nil")
	}
}

func TestInsertAndQueryMetrics(t *testing.T) {
	s := testStore(t)
	now := time.Now().Unix()

	records := []MetricRecord{
		{Timestamp: now, Category: "cpu", Name: "usage", Value: 45.2},
		{Timestamp: now, Category: "cpu", Name: "system", Value: 12.1},
		{Timestamp: now, Category: "memory", Name: "usage_percent", Value: 65.3},
		{Timestamp: now - 60, Category: "cpu", Name: "usage", Value: 40.0},
	}

	if err := s.InsertMetrics(records); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Query all cpu metrics
	results, err := s.QueryMetrics(MetricQuery{Category: "cpu"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 cpu metrics, got %d", len(results))
	}

	// Query with time range
	results, err = s.QueryMetrics(MetricQuery{
		Category: "cpu",
		Name:     "usage",
		Start:    now - 10,
		End:      now + 10,
	})
	if err != nil {
		t.Fatalf("query with range failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 recent cpu usage metric, got %d", len(results))
	}

	// Query with limit
	results, err = s.QueryMetrics(MetricQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 metrics with limit, got %d", len(results))
	}
}

func TestGetLatestMetrics(t *testing.T) {
	s := testStore(t)
	now := time.Now().Unix()

	records := []MetricRecord{
		{Timestamp: now - 60, Category: "cpu", Name: "usage", Value: 40.0},
		{Timestamp: now, Category: "cpu", Name: "usage", Value: 50.0},
		{Timestamp: now - 60, Category: "memory", Name: "used", Value: 4000},
		{Timestamp: now, Category: "memory", Name: "used", Value: 5000},
	}

	if err := s.InsertMetrics(records); err != nil {
		t.Fatal(err)
	}

	latest, err := s.GetLatestMetrics()
	if err != nil {
		t.Fatal(err)
	}

	if len(latest) != 2 {
		t.Fatalf("expected 2 latest metrics, got %d", len(latest))
	}

	for _, m := range latest {
		if m.Category == "cpu" && m.Value != 50.0 {
			t.Errorf("expected latest cpu usage 50, got %f", m.Value)
		}
		if m.Category == "memory" && m.Value != 5000 {
			t.Errorf("expected latest memory used 5000, got %f", m.Value)
		}
	}
}

func TestSystemInfo(t *testing.T) {
	s := testStore(t)

	if err := s.SetSystemInfo("hostname", "test-server"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSystemInfo("os", "Ubuntu 22.04"); err != nil {
		t.Fatal(err)
	}

	info, err := s.GetSystemInfo()
	if err != nil {
		t.Fatal(err)
	}

	if info["hostname"] != "test-server" {
		t.Errorf("hostname = %q, want 'test-server'", info["hostname"])
	}

	// Upsert
	if err := s.SetSystemInfo("hostname", "new-server"); err != nil {
		t.Fatal(err)
	}
	info, err = s.GetSystemInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info["hostname"] != "new-server" {
		t.Errorf("hostname after upsert = %q, want 'new-server'", info["hostname"])
	}
}

func TestCleanup(t *testing.T) {
	s := testStore(t)
	now := time.Now().Unix()

	records := []MetricRecord{
		{Timestamp: now - 86400*40, Category: "cpu", Name: "usage", Value: 10}, // 40 days old
		{Timestamp: now - 86400*35, Category: "cpu", Name: "usage", Value: 20}, // 35 days old
		{Timestamp: now, Category: "cpu", Name: "usage", Value: 50},            // now
	}

	if err := s.InsertMetrics(records); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.Cleanup(30 * 24 * time.Hour) // 30 days
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining, err := s.QueryMetrics(MetricQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}
