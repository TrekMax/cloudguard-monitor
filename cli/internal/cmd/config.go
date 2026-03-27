package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CLIConfig holds the CLI client configuration.
type CLIConfig struct {
	Server   string `yaml:"server"`
	Token    string `yaml:"token"`
	Format   string `yaml:"format"`
	Interval int    `yaml:"interval"` // dashboard refresh interval in seconds
}

func defaultConfig() *CLIConfig {
	return &CLIConfig{
		Server:   "http://localhost:8080",
		Format:   "table",
		Interval: 5,
	}
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudguard")
}

func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

func loadConfig() *CLIConfig {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	yaml.Unmarshal(data, cfg)
	return cfg
}

func saveConfig(cfg *CLIConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath(), data, 0600)
}
