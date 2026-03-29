package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AlertRule represents an alert rule.
type AlertRule struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Duration  int     `json:"duration"`
	Enabled   bool    `json:"enabled"`
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	ID         int64   `json:"id"`
	RuleID     int64   `json:"rule_id"`
	RuleName   string  `json:"rule_name"`
	Status     string  `json:"status"`
	Value      float64 `json:"value"`
	Message    string  `json:"message"`
	FiredAt    int64   `json:"fired_at"`
	ResolvedAt *int64  `json:"resolved_at,omitempty"`
	AckedAt    *int64  `json:"acked_at,omitempty"`
}

// GetAlertRules lists all alert rules.
func (c *Client) GetAlertRules() ([]AlertRule, error) {
	resp, err := c.get("/api/v1/alerts/rules", nil)
	if err != nil {
		return nil, err
	}
	var rules []AlertRule
	if err := json.Unmarshal(resp.Data, &rules); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	return rules, nil
}

// CreateAlertRule creates a new alert rule.
func (c *Client) CreateAlertRule(rule *AlertRule) error {
	return c.post("/api/v1/alerts/rules", rule)
}

// DeleteAlertRule deletes an alert rule.
func (c *Client) DeleteAlertRule(id int64) error {
	return c.delete(fmt.Sprintf("/api/v1/alerts/rules/%d", id))
}

// GetAlerts lists alert events.
func (c *Client) GetAlerts(params map[string]string) ([]AlertEvent, error) {
	resp, err := c.get("/api/v1/alerts", params)
	if err != nil {
		return nil, err
	}
	var events []AlertEvent
	if err := json.Unmarshal(resp.Data, &events); err != nil {
		return nil, fmt.Errorf("parse alerts: %w", err)
	}
	return events, nil
}

// AckAlert acknowledges an alert event.
func (c *Client) AckAlert(id int64) error {
	return c.post(fmt.Sprintf("/api/v1/alerts/%d/ack", id), nil)
}

func (c *Client) post(path string, body interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if apiResp.Code >= 400 {
		return fmt.Errorf("API error %d: %s", apiResp.Code, apiResp.Message)
	}
	return nil
}

func (c *Client) delete(path string) error {
	req, err := http.NewRequest("DELETE", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if apiResp.Code >= 400 {
		return fmt.Errorf("API error %d: %s", apiResp.Code, apiResp.Message)
	}
	return nil
}
