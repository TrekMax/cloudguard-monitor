package alert

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

func testSetup(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	st, err := store.New(dbPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	sched := collector.NewScheduler(logger)
	engine := NewEngine(logger, st, sched)
	return engine, st
}

func TestCheckThreshold(t *testing.T) {
	tests := []struct {
		value     float64
		operator  string
		threshold float64
		expected  bool
	}{
		{90, "gt", 80, true},
		{70, "gt", 80, false},
		{80, "gt", 80, false},
		{10, "lt", 20, true},
		{30, "lt", 20, false},
		{50, "eq", 50, true},
		{50, "eq", 51, false},
		{80, "gte", 80, true},
		{79, "gte", 80, false},
		{80, "lte", 80, true},
		{81, "lte", 80, false},
		{50, "invalid", 50, false},
	}

	for _, tt := range tests {
		got := checkThreshold(tt.value, tt.operator, tt.threshold)
		if got != tt.expected {
			t.Errorf("checkThreshold(%f, %q, %f) = %v, want %v",
				tt.value, tt.operator, tt.threshold, got, tt.expected)
		}
	}
}

func TestAlertRuleCRUD(t *testing.T) {
	_, st := testSetup(t)

	// Create
	rule := &store.AlertRule{
		Name:      "CPU High",
		Category:  "cpu",
		Metric:    "usage",
		Operator:  "gt",
		Threshold: 90,
		Duration:  60,
		Enabled:   true,
	}
	id, err := st.CreateAlertRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Error("expected positive ID")
	}

	// Read
	got, err := st.GetAlertRule(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "CPU High" {
		t.Errorf("name = %q, want 'CPU High'", got.Name)
	}
	if !got.Enabled {
		t.Error("expected enabled")
	}

	// Update
	got.Threshold = 95
	got.Enabled = false
	if err := st.UpdateAlertRule(got); err != nil {
		t.Fatal(err)
	}
	updated, _ := st.GetAlertRule(id)
	if updated.Threshold != 95 {
		t.Errorf("threshold = %f, want 95", updated.Threshold)
	}
	if updated.Enabled {
		t.Error("expected disabled after update")
	}

	// List
	rules, err := st.ListAlertRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	// Delete
	if err := st.DeleteAlertRule(id); err != nil {
		t.Fatal(err)
	}
	rules, _ = st.ListAlertRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestAlertEventLifecycle(t *testing.T) {
	_, st := testSetup(t)

	// Create a rule first
	rule := &store.AlertRule{
		Name: "Mem High", Category: "memory", Metric: "usage_percent",
		Operator: "gt", Threshold: 80, Enabled: true,
	}
	ruleID, _ := st.CreateAlertRule(rule)

	// Create firing event
	event := &store.AlertEvent{
		RuleID:  ruleID,
		Status:  "firing",
		Value:   95.5,
		Message: "memory usage > 80%",
		FiredAt: time.Now().Unix(),
	}
	eventID, err := st.CreateAlertEvent(event)
	if err != nil {
		t.Fatal(err)
	}

	// Get firing event
	firing, err := st.GetFiringEvent(ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if firing.Value != 95.5 {
		t.Errorf("value = %f, want 95.5", firing.Value)
	}

	// Resolve
	if err := st.ResolveAlertEvent(eventID); err != nil {
		t.Fatal(err)
	}

	// List events
	events, err := st.ListAlertEvents(store.AlertEventQuery{Status: "resolved"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 resolved event, got %d", len(events))
	}
	if events[0].ResolvedAt == nil {
		t.Error("expected non-nil resolved_at")
	}

	// Ack
	st.CreateAlertEvent(&store.AlertEvent{
		RuleID: ruleID, Status: "firing", Value: 88, Message: "again", FiredAt: time.Now().Unix(),
	})
	events, _ = st.ListAlertEvents(store.AlertEventQuery{Status: "firing"})
	if len(events) > 0 {
		st.AckAlertEvent(events[0].ID)
		events, _ = st.ListAlertEvents(store.AlertEventQuery{Status: "acknowledged"})
		if len(events) != 1 {
			t.Errorf("expected 1 acked event, got %d", len(events))
		}
	}
}

func TestEngineOnAlert(t *testing.T) {
	engine, st := testSetup(t)

	// Create a rule with threshold 1% (will always fire)
	st.CreateAlertRule(&store.AlertRule{
		Name: "CPU Test", Category: "cpu", Metric: "usage",
		Operator: "gt", Threshold: 1, Duration: 0, Enabled: true,
	})

	var fired bool
	engine.OnAlert(func(event *store.AlertEvent, rule *store.AlertRule) {
		fired = true
	})

	// We need metrics in the scheduler — simulate by running evaluate twice
	// First, manually inject metrics into the scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched := engine.scheduler
	mock := &mockCollector{
		name:     "cpu",
		interval: time.Second,
		metrics: []*collector.Metrics{{
			Category: "cpu",
			Values:   map[string]float64{"usage": 95.0},
		}},
	}
	sched.Register(mock)
	sched.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// First evaluate: starts pending
	engine.evaluate()
	// Second evaluate: fires (duration=0)
	engine.evaluate()

	if !fired {
		t.Error("expected alert to fire")
	}

	sched.Stop()
}

// mockCollector for test
type mockCollector struct {
	name     string
	interval time.Duration
	metrics  []*collector.Metrics
}

func (m *mockCollector) Name() string            { return m.name }
func (m *mockCollector) Interval() time.Duration { return m.interval }
func (m *mockCollector) Collect(ctx context.Context) ([]*collector.Metrics, error) {
	return m.metrics, nil
}
