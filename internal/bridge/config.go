package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Config holds the bridge configuration.
type Config struct {
	NatsURL      string   `yaml:"nats_url"`
	Topics       []string `yaml:"topics,omitempty"`
	Interceptors string   `yaml:"interceptors,omitempty"` // path to interceptors YAML
	Stream       string   `yaml:"stream,omitempty"`       // reuse existing stream name
	Cloud        bool     `yaml:"cloud,omitempty"`        // Phase 2 placeholder
	CloudLeafURL string   `yaml:"cloud_leaf_url,omitempty"`
	Credentials  string   `yaml:"credentials,omitempty"`
}

// InterceptorConfig defines a single interceptor (jq transform).
type InterceptorConfig struct {
	Name string `yaml:"name"`
	From string `yaml:"from"` // source subject filter
	To   string `yaml:"to"`   // destination subject
	JQ   string `yaml:"jq"`   // jq expression
}

// InterceptorsFile is the top-level interceptors YAML.
type InterceptorsFile struct {
	Interceptors []InterceptorConfig `yaml:"interceptors"`
}

// DefaultConfigPath returns ~/.notif/connect.yaml.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notif", "connect.yaml")
}

// DefaultStatusPath returns ~/.notif/connect.status.json.
func DefaultStatusPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notif", "connect.status.json")
}

// DefaultLogPath returns ~/.notif/connect.log.
func DefaultLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notif", "connect.log")
}

// LoadConfig reads config from a YAML file.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes config to a YAML file.
func SaveConfig(cfg *Config, path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadInterceptors reads the interceptors YAML file.
func LoadInterceptors(path string) ([]InterceptorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read interceptors: %w", err)
	}
	var file InterceptorsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse interceptors: %w", err)
	}
	return file.Interceptors, nil
}

// ResolveConfig merges config from flags, env vars, and config file.
// Priority: flags (non-zero) > env > file.
func ResolveConfig(flags *Config, configPath string) (*Config, error) {
	// Start with file config (lowest priority)
	cfg := &Config{}
	if fileCfg, err := LoadConfig(configPath); err == nil {
		*cfg = *fileCfg
	}

	// Apply env vars (middle priority)
	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.NatsURL = v
	}
	if v := os.Getenv("NOTIF_CLOUD_URL"); v != "" {
		cfg.CloudLeafURL = v
	}

	// Apply flags (highest priority, only non-zero values)
	if flags.NatsURL != "" {
		cfg.NatsURL = flags.NatsURL
	}
	if len(flags.Topics) > 0 {
		cfg.Topics = flags.Topics
	}
	if flags.Interceptors != "" {
		cfg.Interceptors = flags.Interceptors
	}
	if flags.Stream != "" {
		cfg.Stream = flags.Stream
	}
	if flags.Cloud {
		cfg.Cloud = flags.Cloud
	}

	return cfg, nil
}

// DetectDrift compares running config against the file on disk.
// Returns a list of field names that differ, or nil if identical.
func DetectDrift(running *Config, configPath string) []string {
	fileCfg, err := LoadConfig(configPath)
	if err != nil {
		return nil // can't detect drift if file is unreadable
	}

	var drifted []string
	rv := reflect.ValueOf(running).Elem()
	fv := reflect.ValueOf(fileCfg).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !reflect.DeepEqual(rv.Field(i).Interface(), fv.Field(i).Interface()) {
			drifted = append(drifted, field.Tag.Get("yaml"))
		}
	}
	return drifted
}
