package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func jsonResponse(code int, msg string, data interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dataBytes, _ := json.Marshal(data)
		resp := map[string]interface{}{
			"code":      code,
			"message":   msg,
			"data":      json.RawMessage(dataBytes),
			"timestamp": 1234567890,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func TestHealth(t *testing.T) {
	srv := mockServer(jsonResponse(200, "success", map[string]string{"status": "ok"}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Health(); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestHealth_Error(t *testing.T) {
	c := New("http://localhost:1", "")
	if err := c.Health(); err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestGetStatus(t *testing.T) {
	data := map[string]interface{}{
		"cpu":     map[string]float64{"usage": 45.2, "cores": 4},
		"memory":  map[string]float64{"total": 8192, "used": 4096, "usage_percent": 50},
		"network": map[string]float64{"rx_bytes_rate": 1024, "connections": 10},
	}

	srv := mockServer(jsonResponse(200, "success", data))
	defer srv.Close()

	c := New(srv.URL, "")
	status, err := c.GetStatus()
	if err != nil {
		t.Fatal(err)
	}

	if status.CPU["usage"] != 45.2 {
		t.Errorf("cpu usage = %f, want 45.2", status.CPU["usage"])
	}
}

func TestGetSystem(t *testing.T) {
	data := SystemInfo{
		Hostname: "test-host",
		OS:       "Linux",
		Arch:     "amd64",
		CPUCores: 4,
	}

	srv := mockServer(jsonResponse(200, "success", data))
	defer srv.Close()

	c := New(srv.URL, "")
	info, err := c.GetSystem()
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname != "test-host" {
		t.Errorf("hostname = %q, want 'test-host'", info.Hostname)
	}
}

func TestGetMetrics_Empty(t *testing.T) {
	srv := mockServer(jsonResponse(200, "success", nil))
	defer srv.Close()

	c := New(srv.URL, "")
	records, err := c.GetMetrics(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := mockServer(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		jsonResponse(200, "success", nil)(w, r)
	})
	defer srv.Close()

	c := New(srv.URL, "my-secret-token")
	c.Health()

	if gotAuth != "Bearer my-secret-token" {
		t.Errorf("auth header = %q, want 'Bearer my-secret-token'", gotAuth)
	}
}

func TestAPIError(t *testing.T) {
	srv := mockServer(jsonResponse(401, "invalid token", nil))
	defer srv.Close()

	c := New(srv.URL, "wrong-token")
	err := c.Health()
	if err == nil {
		t.Error("expected error for 401 response")
	}
}
