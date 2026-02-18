package interceptor

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the list of interceptor configurations.
type Config struct {
	Interceptors []InterceptorConfig `yaml:"interceptors"`
}

// InterceptorConfig defines a single interceptor.
type InterceptorConfig struct {
	Name    string `yaml:"name"`
	From    string `yaml:"from"`
	To      string `yaml:"to"`
	Jq      string `yaml:"jq"`
	Enabled *bool  `yaml:"enabled"` // defaults to true if nil
}

// IsEnabled returns whether this interceptor is enabled (defaults to true).
func (c InterceptorConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// LoadConfig reads a YAML file and returns the parsed Config.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read interceptor config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse interceptor config: %w", err)
	}
	return &cfg, nil
}
