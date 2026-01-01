package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the CLI configuration.
type Config struct {
	APIKey string `json:"api_key,omitempty"`
	Server string `json:"server,omitempty"`
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notif", "config.json")
}

// Load reads config from the specified path or default.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes config to the specified path or default.
func Save(cfg *Config, path string) error {
	if path == "" {
		path = DefaultPath()
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
