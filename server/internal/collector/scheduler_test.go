package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// mockCollector is a simple collector for testing the scheduler.
type mockCollector struct {
	name     string
	interval time.Duration
	metrics  []*Metrics
	err      error
	called   int
}

func (m *mockCollector) Name() string            { return m.name }
func (m *mockCollector) Interval() time.Duration { return m.interval }
func (m *mockCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	m.called++
	return m.metrics, m.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestScheduler_Register(t *testing.T) {
	s := NewScheduler(testLogger())
	mock := &mockCollector{name: "test", interval: time.Second}
	s.Register(mock)

	if len(s.collectors) != 1 {
		t.Errorf("expected 1 collector, got %d", len(s.collectors))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler(testLogger())
	mock := &mockCollector{
		name:     "test",
		interval: 50 * time.Millisecond,
		metrics: []*Metrics{{
			Category:  "test",
			Timestamp: time.Now(),
			Values:    map[string]float64{"value": 42},
		}},
	}
	s.Register(mock)

	ctx := context.Background()
	s.Start(ctx)

	// Wait for at least 2 collections
	time.Sleep(150 * time.Millisecond)
	s.Stop()

	if mock.called < 2 {
		t.Errorf("expected at least 2 calls, got %d", mock.called)
	}

	snap := s.Latest()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Metrics["test"]) != 1 {
		t.Errorf("expected 1 metric in snapshot, got %d", len(snap.Metrics["test"]))
	}
}

func TestScheduler_MultipleCollectors(t *testing.T) {
	s := NewScheduler(testLogger())
	mock1 := &mockCollector{
		name:     "cpu",
		interval: 50 * time.Millisecond,
		metrics:  []*Metrics{{Category: "cpu", Values: map[string]float64{"usage": 50}}},
	}
	mock2 := &mockCollector{
		name:     "memory",
		interval: 50 * time.Millisecond,
		metrics:  []*Metrics{{Category: "memory", Values: map[string]float64{"used": 4096}}},
	}
	s.Register(mock1)
	s.Register(mock2)

	ctx := context.Background()
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	snap := s.Latest()
	if len(snap.Metrics) != 2 {
		t.Errorf("expected 2 collector entries, got %d", len(snap.Metrics))
	}
}

func TestScheduler_LatestEmpty(t *testing.T) {
	s := NewScheduler(testLogger())
	snap := s.Latest()
	if snap == nil {
		t.Fatal("expected non-nil snapshot even before start")
	}
	if len(snap.Metrics) != 0 {
		t.Errorf("expected empty metrics, got %d", len(snap.Metrics))
	}
}
