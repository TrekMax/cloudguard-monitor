package alert

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

// EventCallback is called when an alert event is created or resolved.
type EventCallback func(event *store.AlertEvent, rule *store.AlertRule)

// Engine evaluates alert rules against collected metrics.
type Engine struct {
	logger    *slog.Logger
	store     *store.Store
	scheduler *collector.Scheduler

	mu             sync.Mutex
	pendingSince   map[int64]time.Time // rule_id -> first breach time
	suppressUntil  map[int64]time.Time // rule_id -> suppress notifications until
	suppressionMin int                 // minutes to suppress duplicate alerts

	onAlert EventCallback
}

// NewEngine creates a new alert engine.
func NewEngine(logger *slog.Logger, st *store.Store, sched *collector.Scheduler) *Engine {
	return &Engine{
		logger:         logger,
		store:          st,
		scheduler:      sched,
		pendingSince:   make(map[int64]time.Time),
		suppressUntil:  make(map[int64]time.Time),
		suppressionMin: 5,
	}
}

// OnAlert registers a callback for alert events.
func (e *Engine) OnAlert(cb EventCallback) {
	e.onAlert = cb
}

// Run starts the alert evaluation loop.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluate()
		}
	}
}

func (e *Engine) evaluate() {
	rules, err := e.store.ListAlertRules()
	if err != nil {
		e.logger.Error("failed to list alert rules", "error", err)
		return
	}

	snap := e.scheduler.Latest()
	now := time.Now()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		value, ok := e.getMetricValue(snap, rule.Category, rule.Metric)
		if !ok {
			continue
		}

		breached := checkThreshold(value, rule.Operator, rule.Threshold)

		e.mu.Lock()
		if breached {
			e.handleBreach(rule, value, now)
		} else {
			e.handleRecovery(rule, value, now)
		}
		e.mu.Unlock()
	}
}

func (e *Engine) handleBreach(rule store.AlertRule, value float64, now time.Time) {
	// Track when the breach started
	since, pending := e.pendingSince[rule.ID]
	if !pending {
		e.pendingSince[rule.ID] = now
		return // Just started breaching, wait for duration
	}

	// Check if duration requirement is met
	if rule.Duration > 0 && now.Sub(since).Seconds() < float64(rule.Duration) {
		return // Still within grace period
	}

	// Check if already firing
	existing, err := e.store.GetFiringEvent(rule.ID)
	if err == nil && existing != nil {
		return // Already has a firing event
	}

	// Check suppression
	if suppress, ok := e.suppressUntil[rule.ID]; ok && now.Before(suppress) {
		return
	}

	// Fire alert
	msg := fmt.Sprintf("%s %s %s %.2f (current: %.2f) for %ds",
		rule.Category, rule.Metric, rule.Operator, rule.Threshold, value, rule.Duration)

	event := &store.AlertEvent{
		RuleID:  rule.ID,
		Status:  "firing",
		Value:   value,
		Message: msg,
		FiredAt: now.Unix(),
	}

	id, err := e.store.CreateAlertEvent(event)
	if err != nil {
		e.logger.Error("failed to create alert event", "error", err)
		return
	}
	event.ID = id

	// Set suppression
	e.suppressUntil[rule.ID] = now.Add(time.Duration(e.suppressionMin) * time.Minute)

	e.logger.Warn("alert fired",
		"rule", rule.Name,
		"value", value,
		"threshold", rule.Threshold,
	)

	if e.onAlert != nil {
		e.onAlert(event, &rule)
	}
}

func (e *Engine) handleRecovery(rule store.AlertRule, value float64, now time.Time) {
	// Clear pending state
	delete(e.pendingSince, rule.ID)

	// Check if there's a firing event to resolve
	existing, err := e.store.GetFiringEvent(rule.ID)
	if err == sql.ErrNoRows || existing == nil {
		return
	}
	if err != nil {
		return
	}

	if err := e.store.ResolveAlertEvent(existing.ID); err != nil {
		e.logger.Error("failed to resolve alert event", "error", err)
		return
	}

	e.logger.Info("alert resolved",
		"rule", rule.Name,
		"value", value,
	)

	resolved := &store.AlertEvent{
		ID:      existing.ID,
		RuleID:  rule.ID,
		Status:  "resolved",
		Value:   value,
		Message: fmt.Sprintf("%s %s recovered (current: %.2f)", rule.Category, rule.Metric, value),
		FiredAt: existing.FiredAt,
	}
	resolvedAt := now.Unix()
	resolved.ResolvedAt = &resolvedAt

	if e.onAlert != nil {
		e.onAlert(resolved, &rule)
	}
}

func (e *Engine) getMetricValue(snap *collector.Snapshot, category, metric string) (float64, bool) {
	metrics, ok := snap.Metrics[category]
	if !ok {
		return 0, false
	}

	for _, m := range metrics {
		if m.Category == category {
			if v, ok := m.Values[metric]; ok {
				return v, true
			}
		}
	}
	return 0, false
}

func checkThreshold(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "lt":
		return value < threshold
	case "eq":
		return value == threshold
	case "gte":
		return value >= threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}
