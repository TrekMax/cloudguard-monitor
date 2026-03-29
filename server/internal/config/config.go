package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for CloudGuard Monitor.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Collector CollectorConfig `yaml:"collector"`
	Database  DatabaseConfig  `yaml:"database"`
	Log       LogConfig       `yaml:"log"`
	Auth      AuthConfig      `yaml:"auth"`
	TLS       TLSConfig       `yaml:"tls"`
	Security  SecurityConfig  `yaml:"security"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	AutoCert bool   `yaml:"auto_cert"`
}

type SecurityConfig struct {
	IPWhitelist []string `yaml:"ip_whitelist"`
}

type CollectorConfig struct {
	CPUInterval     int `yaml:"cpu_interval"`     // seconds
	MemoryInterval  int `yaml:"memory_interval"`  // seconds
	DiskInterval    int `yaml:"disk_interval"`    // seconds
	NetworkInterval int `yaml:"network_interval"` // seconds
}

type DatabaseConfig struct {
	Path          string `yaml:"path"`
	RetentionDays int    `yaml:"retention_days"`
}

type LogConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // text, json
}

type AuthConfig struct {
	Token string `yaml:"token"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Collector: CollectorConfig{
			CPUInterval:     5,
			MemoryInterval:  5,
			DiskInterval:    30,
			NetworkInterval: 5,
		},
		Database: DatabaseConfig{
			Path:          "./data/cloudguard.db",
			RetentionDays: 30,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load reads configuration from the given file path.
// If path is empty, it tries default locations, then falls back to defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		// Try default locations
		candidates := []string{
			"cloudguard.yaml",
			"/etc/cloudguard/cloudguard.yaml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
	}

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}
