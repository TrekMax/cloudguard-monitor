package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/store"
	"github.com/trekmax/cloudguard-monitor/internal/ws"
)

func setupTestServer(t *testing.T, token string) (*Server, *store.Store) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.New(dbPath, logger)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	sched := collector.NewScheduler(logger)
	hub := ws.NewHub(logger)
	sysInfo := &collector.SystemInfo{
		Hostname:     "test-host",
		OS:           "Linux",
		Arch:         "amd64",
		CPUCores:     4,
		AgentVersion: "0.1.0",
	}

	srv := NewServer(logger, st, sched, sysInfo, token, hub)
	return srv, st
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("health status = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 200 {
		t.Errorf("resp code = %d, want 200", resp.Code)
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	srv, _ := setupTestServer(t, "secret-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	srv, _ := setupTestServer(t, "secret-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	srv, _ := setupTestServer(t, "secret-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_EmptyTokenRejects(t *testing.T) {
	srv, _ := setupTestServer(t, "")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("status = %d, want 503 (auth not configured)", w.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status code = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 200 {
		t.Errorf("resp code = %d, want 200", resp.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv, st := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	// Insert some test data
	now := time.Now().Unix()
	st.InsertMetrics([]store.MetricRecord{
		{Timestamp: now, Category: "cpu", Name: "usage", Value: 45.2},
		{Timestamp: now, Category: "memory", Name: "used", Value: 4096},
	})

	// Query all
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Query by category
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/metrics?category=cpu", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 cpu metric, got %d", len(data))
	}
}

func TestMetricsEndpoint_InvalidParams(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	tests := []struct {
		query string
	}{
		{"/api/v1/metrics?start=abc"},
		{"/api/v1/metrics?end=abc"},
		{"/api/v1/metrics?limit=abc"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", tt.query, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("query %q: status = %d, want 400", tt.query, w.Code)
		}
	}
}

func TestSystemEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/system", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be an object")
	}
	if data["hostname"] != "test-host" {
		t.Errorf("hostname = %v, want 'test-host'", data["hostname"])
	}
}

func TestProcessesEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/processes", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAlertRulesAPI(t *testing.T) {
	srv, _ := setupTestServer(t, "test-token")
	router := srv.SetupRouter()
	auth := "Bearer test-token"

	// Create rule
	body := `{"name":"CPU High","category":"cpu","metric":"usage","operator":"gt","threshold":90,"duration":60}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/alerts/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("create rule: status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	// List rules
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/alerts/rules", nil)
	req.Header.Set("Authorization", auth)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("list rules: status = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	rules, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	// Delete rule
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/alerts/rules/1", nil)
	req.Header.Set("Authorization", auth)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("delete rule: status = %d, want 200", w.Code)
	}
}

func TestAlertEventsAPI(t *testing.T) {
	srv, st := setupTestServer(t, "test-token")
	router := srv.SetupRouter()
	auth := "Bearer test-token"

	// Create a rule and event directly
	ruleID, _ := st.CreateAlertRule(&store.AlertRule{
		Name: "Test", Category: "cpu", Metric: "usage",
		Operator: "gt", Threshold: 90, Enabled: true,
	})
	st.CreateAlertEvent(&store.AlertEvent{
		RuleID: ruleID, Status: "firing", Value: 95, Message: "test", FiredAt: time.Now().Unix(),
	})

	// List alerts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/alerts?status=firing", nil)
	req.Header.Set("Authorization", auth)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("list alerts: status = %d, want 200", w.Code)
	}

	// Ack alert
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/alerts/1/ack", nil)
	req.Header.Set("Authorization", auth)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("ack alert: status = %d, want 200", w.Code)
	}
}
