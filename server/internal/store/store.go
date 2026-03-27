package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages the SQLite database for metrics storage.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
	mu     sync.Mutex
}

// MetricRecord represents a single stored metric data point.
type MetricRecord struct {
	ID        int64   `json:"id"`
	Timestamp int64   `json:"timestamp"`
	Category  string  `json:"category"`
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Labels    string  `json:"labels,omitempty"`
}

// New opens or creates a SQLite database at the given path.
func New(dbPath string, logger *slog.Logger) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Limit connections for SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &Store{db: db, logger: logger}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS metrics (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp INTEGER NOT NULL,
			category  TEXT NOT NULL,
			name      TEXT NOT NULL,
			value     REAL NOT NULL,
			labels    TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_ts_cat ON metrics(timestamp, category)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_cat_name ON metrics(category, name)`,

		`CREATE TABLE IF NOT EXISTS alert_rules (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL,
			category   TEXT NOT NULL,
			metric     TEXT NOT NULL,
			operator   TEXT NOT NULL,
			threshold  REAL NOT NULL,
			duration   INTEGER DEFAULT 0,
			enabled    INTEGER DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS alert_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id     INTEGER NOT NULL,
			status      TEXT NOT NULL,
			value       REAL NOT NULL,
			message     TEXT,
			fired_at    INTEGER NOT NULL,
			resolved_at INTEGER,
			acked_at    INTEGER,
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id)
		)`,

		`CREATE TABLE IF NOT EXISTS system_info (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	return nil
}

// InsertMetrics inserts a batch of metric records.
func (s *Store) InsertMetrics(records []MetricRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO metrics (timestamp, category, name, value, labels) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range records {
		if _, err := stmt.Exec(r.Timestamp, r.Category, r.Name, r.Value, r.Labels); err != nil {
			return fmt.Errorf("exec insert: %w", err)
		}
	}

	return tx.Commit()
}

// QueryMetrics retrieves metrics filtered by category, name, and time range.
type MetricQuery struct {
	Category string
	Name     string
	Start    int64 // unix timestamp
	End      int64 // unix timestamp
	Limit    int
}

func (s *Store) QueryMetrics(q MetricQuery) ([]MetricRecord, error) {
	query := "SELECT id, timestamp, category, name, value, labels FROM metrics WHERE 1=1"
	var args []interface{}

	if q.Category != "" {
		query += " AND category = ?"
		args = append(args, q.Category)
	}
	if q.Name != "" {
		query += " AND name = ?"
		args = append(args, q.Name)
	}
	if q.Start > 0 {
		query += " AND timestamp >= ?"
		args = append(args, q.Start)
	}
	if q.End > 0 {
		query += " AND timestamp <= ?"
		args = append(args, q.End)
	}

	query += " ORDER BY timestamp DESC"

	if q.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, q.Limit)
	} else {
		query += " LIMIT 1000"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	var results []MetricRecord
	for rows.Next() {
		var r MetricRecord
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Category, &r.Name, &r.Value, &r.Labels); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// GetLatestMetrics returns the most recent metric for each category+name combination.
func (s *Store) GetLatestMetrics() ([]MetricRecord, error) {
	query := `SELECT m.id, m.timestamp, m.category, m.name, m.value, m.labels
		FROM metrics m
		INNER JOIN (
			SELECT category, name, MAX(timestamp) as max_ts
			FROM metrics
			GROUP BY category, name
		) latest ON m.category = latest.category AND m.name = latest.name AND m.timestamp = latest.max_ts`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query latest: %w", err)
	}
	defer rows.Close()

	var results []MetricRecord
	for rows.Next() {
		var r MetricRecord
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Category, &r.Name, &r.Value, &r.Labels); err != nil {
			return nil, fmt.Errorf("scan latest: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// Downsample aggregates old data into lower resolution.
func (s *Store) Downsample(olderThan time.Duration, intervalSecs int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan).Unix()

	// Create aggregated records
	query := `INSERT INTO metrics (timestamp, category, name, value, labels)
		SELECT (timestamp / ? * ?) as bucket_ts, category, name, AVG(value), ''
		FROM metrics
		WHERE timestamp < ? AND labels = ''
		GROUP BY bucket_ts, category, name
		HAVING COUNT(*) > 1`

	result, err := s.db.Exec(query, intervalSecs, intervalSecs, cutoff)
	if err != nil {
		return 0, fmt.Errorf("downsample insert: %w", err)
	}

	inserted, _ := result.RowsAffected()

	// Delete original records that were aggregated
	_, err = s.db.Exec(`DELETE FROM metrics WHERE timestamp < ? AND labels = '' AND id NOT IN (
		SELECT MAX(id) FROM metrics WHERE timestamp < ? GROUP BY (timestamp / ? * ?), category, name
	)`, cutoff, cutoff, intervalSecs, intervalSecs)
	if err != nil {
		return inserted, fmt.Errorf("downsample delete: %w", err)
	}

	return inserted, nil
}

// Cleanup removes metrics older than the given duration.
func (s *Store) Cleanup(olderThan time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan).Unix()
	result, err := s.db.Exec("DELETE FROM metrics WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup: %w", err)
	}

	return result.RowsAffected()
}

// SetSystemInfo upserts a system info key-value pair.
func (s *Store) SetSystemInfo(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO system_info (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().Unix(),
	)
	return err
}

// GetSystemInfo retrieves all system info key-value pairs.
func (s *Store) GetSystemInfo() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM system_info")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	info := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		info[k] = v
	}
	return info, rows.Err()
}
