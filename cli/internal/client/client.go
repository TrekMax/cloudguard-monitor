package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// APIResponse is the unified server response format.
type APIResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// Client is the HTTP client for CloudGuard Monitor API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New creates a new API client.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) get(path string, params map[string]string) (*APIResponse, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("API error %d: %s", apiResp.Code, apiResp.Message)
	}

	return &apiResp, nil
}

// StatusData represents the /api/v1/status response (normalized).
type StatusData struct {
	CPU     map[string]float64
	Memory  map[string]float64
	Network map[string]float64
}

// GetStatus fetches real-time server status.
func (c *Client) GetStatus() (*StatusData, error) {
	resp, err := c.get("/api/v1/status", nil)
	if err != nil {
		return nil, err
	}

	// Parse into raw map to handle mixed types (object vs array)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &raw); err != nil {
		return nil, fmt.Errorf("parse status data: %w", err)
	}

	data := &StatusData{
		CPU:     parseMetricField(raw["cpu"]),
		Memory:  parseMetricField(raw["memory"]),
		Network: parseMetricField(raw["network"]),
	}

	return data, nil
}

// parseMetricField extracts a map[string]float64 from either an object or
// an array (takes the first element without labels as the aggregate).
func parseMetricField(raw json.RawMessage) map[string]float64 {
	if len(raw) == 0 {
		return nil
	}

	// Try as a plain object first
	var obj map[string]float64
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj
	}

	// Try as an array — pick the first entry whose "labels" is empty/missing
	var arr []struct {
		Values map[string]float64 `json:"values"`
		Labels map[string]string  `json:"labels"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			if len(item.Labels) == 0 {
				return item.Values
			}
		}
		// Fallback: return the first item's values
		if len(arr) > 0 {
			return arr[0].Values
		}
	}

	return nil
}

// SystemInfo represents the /api/v1/system response.
type SystemInfo struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Platform     string `json:"platform"`
	Kernel       string `json:"kernel"`
	Arch         string `json:"arch"`
	CPUCores     int    `json:"cpu_cores"`
	BootTime     int64  `json:"boot_time"`
	Uptime       int64  `json:"uptime"`
	GoVersion    string `json:"go_version"`
	AgentVersion string `json:"agent_version"`
}

// GetSystem fetches system information.
func (c *Client) GetSystem() (*SystemInfo, error) {
	resp, err := c.get("/api/v1/system", nil)
	if err != nil {
		return nil, err
	}

	var data SystemInfo
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parse system data: %w", err)
	}
	return &data, nil
}

// MetricRecord represents a stored metric data point.
type MetricRecord struct {
	ID        int64   `json:"id"`
	Timestamp int64   `json:"timestamp"`
	Category  string  `json:"category"`
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Labels    string  `json:"labels,omitempty"`
}

// GetMetrics fetches historical metrics.
func (c *Client) GetMetrics(params map[string]string) ([]MetricRecord, error) {
	resp, err := c.get("/api/v1/metrics", params)
	if err != nil {
		return nil, err
	}

	var data []MetricRecord
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		// Could be null
		if string(resp.Data) == "null" {
			return nil, nil
		}
		return nil, fmt.Errorf("parse metrics data: %w", err)
	}
	return data, nil
}

// Health checks server connectivity.
func (c *Client) Health() error {
	_, err := c.get("/health", nil)
	return err
}
