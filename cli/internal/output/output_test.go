package output

import (
	"bytes"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"yaml", FormatYAML},
		{"YAML", FormatYAML},
		{"table", FormatTable},
		{"", FormatTable},
		{"unknown", FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseFormat(tt.input)
			if got != tt.expected {
				t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	if err := PrintJSON(&buf, data); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty JSON output")
	}
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	if err := PrintYAML(&buf, data); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty YAML output")
	}
}

func TestFromStatusData(t *testing.T) {
	cpu := map[string]float64{"usage": 45.2, "cores": 8}
	mem := map[string]float64{"total": 8589934592, "used": 4294967296, "usage_percent": 50}
	net := map[string]float64{"rx_bytes_rate": 1024, "tx_bytes_rate": 512, "connections": 42}

	s := FromStatusData(cpu, mem, net)

	if s.CPU.Usage != 45.2 {
		t.Errorf("CPU usage = %f, want 45.2", s.CPU.Usage)
	}
	if s.Memory.UsagePercent != 50 {
		t.Errorf("Memory usage = %f, want 50", s.Memory.UsagePercent)
	}
	if s.Network.Connections != 42 {
		t.Errorf("Connections = %f, want 42", s.Network.Connections)
	}
}
