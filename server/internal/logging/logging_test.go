package logging

import (
	"log/slog"
	"testing"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		level  string
		format string
	}{
		{"debug", "text"},
		{"info", "text"},
		{"warn", "json"},
		{"error", "json"},
		{"unknown", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.level+"_"+tt.format, func(t *testing.T) {
			logger := Setup(tt.level, tt.format)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
