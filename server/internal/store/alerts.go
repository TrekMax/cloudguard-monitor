package store

import (
	"fmt"
	"time"
)

// AlertRule represents an alert rule configuration.
type AlertRule struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"` // gt, lt, eq
	Threshold float64 `json:"threshold"`
	Duration  int     `json:"duration"` // seconds, 0 = instant
	Enabled   bool    `json:"enabled"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	ID         int64   `json:"id"`
	RuleID     int64   `json:"rule_id"`
	RuleName   string  `json:"rule_name,omitempty"`
	Status     string  `json:"status"` // firing, resolved, acknowledged
	Value      float64 `json:"value"`
	Message    string  `json:"message"`
	FiredAt    int64   `json:"fired_at"`
	ResolvedAt *int64  `json:"resolved_at,omitempty"`
	AckedAt    *int64  `json:"acked_at,omitempty"`
}

// CreateAlertRule inserts a new alert rule.
func (s *Store) CreateAlertRule(r *AlertRule) (int64, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(
		`INSERT INTO alert_rules (name, category, metric, operator, threshold, duration, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Category, r.Metric, r.Operator, r.Threshold, r.Duration, boolToInt(r.Enabled), now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("create alert rule: %w", err)
	}
	return result.LastInsertId()
}

// UpdateAlertRule updates an existing alert rule.
func (s *Store) UpdateAlertRule(r *AlertRule) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`UPDATE alert_rules SET name=?, category=?, metric=?, operator=?, threshold=?, duration=?, enabled=?, updated_at=? WHERE id=?`,
		r.Name, r.Category, r.Metric, r.Operator, r.Threshold, r.Duration, boolToInt(r.Enabled), now, r.ID,
	)
	return err
}

// DeleteAlertRule deletes an alert rule.
func (s *Store) DeleteAlertRule(id int64) error {
	_, err := s.db.Exec("DELETE FROM alert_rules WHERE id=?", id)
	return err
}

// GetAlertRule retrieves a single alert rule.
func (s *Store) GetAlertRule(id int64) (*AlertRule, error) {
	row := s.db.QueryRow(
		"SELECT id, name, category, metric, operator, threshold, duration, enabled, created_at, updated_at FROM alert_rules WHERE id=?", id,
	)
	var r AlertRule
	var enabled int
	err := row.Scan(&r.ID, &r.Name, &r.Category, &r.Metric, &r.Operator, &r.Threshold, &r.Duration, &enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.Enabled = enabled != 0
	return &r, nil
}

// ListAlertRules returns all alert rules.
func (s *Store) ListAlertRules() ([]AlertRule, error) {
	rows, err := s.db.Query(
		"SELECT id, name, category, metric, operator, threshold, duration, enabled, created_at, updated_at FROM alert_rules ORDER BY id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var r AlertRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Category, &r.Metric, &r.Operator, &r.Threshold, &r.Duration, &enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// CreateAlertEvent inserts a new alert event.
func (s *Store) CreateAlertEvent(e *AlertEvent) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO alert_events (rule_id, status, value, message, fired_at) VALUES (?, ?, ?, ?, ?)`,
		e.RuleID, e.Status, e.Value, e.Message, e.FiredAt,
	)
	if err != nil {
		return 0, fmt.Errorf("create alert event: %w", err)
	}
	return result.LastInsertId()
}

// ResolveAlertEvent marks a firing event as resolved.
func (s *Store) ResolveAlertEvent(id int64) error {
	now := time.Now().Unix()
	_, err := s.db.Exec("UPDATE alert_events SET status='resolved', resolved_at=? WHERE id=?", now, id)
	return err
}

// AckAlertEvent marks an event as acknowledged.
func (s *Store) AckAlertEvent(id int64) error {
	now := time.Now().Unix()
	_, err := s.db.Exec("UPDATE alert_events SET status='acknowledged', acked_at=? WHERE id=?", now, id)
	return err
}

// GetFiringEvent returns the active firing event for a given rule, if any.
func (s *Store) GetFiringEvent(ruleID int64) (*AlertEvent, error) {
	row := s.db.QueryRow(
		`SELECT id, rule_id, status, value, message, fired_at, resolved_at, acked_at
		FROM alert_events WHERE rule_id=? AND status='firing' ORDER BY fired_at DESC LIMIT 1`, ruleID,
	)
	var e AlertEvent
	err := row.Scan(&e.ID, &e.RuleID, &e.Status, &e.Value, &e.Message, &e.FiredAt, &e.ResolvedAt, &e.AckedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// AlertEventQuery holds filters for listing alert events.
type AlertEventQuery struct {
	Status string
	RuleID int64
	Limit  int
	Offset int
}

// ListAlertEvents returns alert events with optional filters.
func (s *Store) ListAlertEvents(q AlertEventQuery) ([]AlertEvent, error) {
	query := `SELECT e.id, e.rule_id, r.name, e.status, e.value, e.message, e.fired_at, e.resolved_at, e.acked_at
		FROM alert_events e LEFT JOIN alert_rules r ON e.rule_id = r.id WHERE 1=1`
	var args []interface{}

	if q.Status != "" {
		query += " AND e.status = ?"
		args = append(args, q.Status)
	}
	if q.RuleID > 0 {
		query += " AND e.rule_id = ?"
		args = append(args, q.RuleID)
	}

	query += " ORDER BY e.fired_at DESC"

	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	query += " LIMIT ?"
	args = append(args, limit)

	if q.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, q.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		var e AlertEvent
		var ruleName *string
		if err := rows.Scan(&e.ID, &e.RuleID, &ruleName, &e.Status, &e.Value, &e.Message, &e.FiredAt, &e.ResolvedAt, &e.AckedAt); err != nil {
			return nil, err
		}
		if ruleName != nil {
			e.RuleName = *ruleName
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
