package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		wantSec int64
		wantErr bool
	}{
		{"30s", 30, false},
		{"5m", 300, false},
		{"2h", 7200, false},
		{"7d", 604800, false},
		{"1h30m", 5400, false},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && int64(got.Seconds()) != tt.wantSec {
				t.Errorf("parseDuration(%q) = %v sec, want %v sec", tt.input, int64(got.Seconds()), tt.wantSec)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%f) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Use a temp dir as home
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := &CLIConfig{
		Server:   "http://test:9090",
		Token:    "test-token",
		Format:   "json",
		Interval: 3,
	}

	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Verify file exists
	cfgPath := filepath.Join(tmpDir, ".cloudguard", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not found: %v", err)
	}

	loaded := loadConfig()
	if loaded.Server != "http://test:9090" {
		t.Errorf("server = %q, want 'http://test:9090'", loaded.Server)
	}
	if loaded.Token != "test-token" {
		t.Errorf("token = %q, want 'test-token'", loaded.Token)
	}
	if loaded.Interval != 3 {
		t.Errorf("interval = %d, want 3", loaded.Interval)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Server != "http://localhost:8080" {
		t.Errorf("default server = %q, want 'http://localhost:8080'", cfg.Server)
	}
	if cfg.Format != "table" {
		t.Errorf("default format = %q, want 'table'", cfg.Format)
	}
}
