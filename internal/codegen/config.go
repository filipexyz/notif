// Package codegen provides code generation from JSON Schema.
package codegen

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the .notif.yaml configuration file.
type Config struct {
	Version int           `yaml:"version"`
	Server  string        `yaml:"server,omitempty"`
	Output  OutputConfig  `yaml:"output"`
	Options OptionsConfig `yaml:"options,omitempty"`
	Schemas SchemaList    `yaml:"schemas"`
}

// SchemaList can be either "all" (fetch all from server) or a list of schema entries.
type SchemaList struct {
	All     bool          // If true, fetch all schemas from server
	Entries []SchemaEntry // Explicit list of schemas
}

// UnmarshalYAML implements custom unmarshaling for SchemaList.
func (s *SchemaList) UnmarshalYAML(value *yaml.Node) error {
	// Check if it's the string "all"
	if value.Kind == yaml.ScalarNode && value.Value == "all" {
		s.All = true
		return nil
	}

	// Otherwise, parse as array of schema entries
	return value.Decode(&s.Entries)
}

// MarshalYAML implements custom marshaling for SchemaList.
func (s SchemaList) MarshalYAML() (interface{}, error) {
	if s.All {
		return "all", nil
	}
	return s.Entries, nil
}

// OutputConfig specifies output directories per language.
type OutputConfig struct {
	TypeScript string `yaml:"typescript,omitempty"`
	Go         string `yaml:"go,omitempty"`
}

// OptionsConfig holds language-specific generation options.
type OptionsConfig struct {
	TypeScript TypeScriptOptions `yaml:"typescript,omitempty"`
	Go         GoOptions         `yaml:"go,omitempty"`
}

// TypeScriptOptions holds TypeScript generation options.
type TypeScriptOptions struct {
	Exports string `yaml:"exports,omitempty"` // named | default
}

// GoOptions holds Go generation options.
type GoOptions struct {
	Package  string `yaml:"package,omitempty"`  // Package name for generated code
	JSONTags string `yaml:"jsonTags,omitempty"` // omitempty | required | none
}

// SchemaEntry represents a schema to generate.
// Can be a simple string (schema name) or a detailed config.
type SchemaEntry struct {
	Name      string   `yaml:"name,omitempty"`
	Languages []string `yaml:"languages,omitempty"`
	File      string   `yaml:"file,omitempty"` // Local file instead of fetching from server
}

// UnmarshalYAML implements custom unmarshaling for SchemaEntry to support both
// simple string format and full object format.
func (s *SchemaEntry) UnmarshalYAML(value *yaml.Node) error {
	// Try simple string format first
	if value.Kind == yaml.ScalarNode {
		s.Name = value.Value
		return nil
	}

	// Full object format
	type schemaEntryAlias SchemaEntry
	return value.Decode((*schemaEntryAlias)(s))
}

// LoadConfig loads a .notif.yaml configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	cfg.applyDefaults()

	return &cfg, nil
}

// FindConfig searches for .notif.yaml in the current directory and parent directories.
func FindConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ".notif.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".notif.yaml not found in current directory or any parent directory")
}

func (c *Config) validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (expected 1)", c.Version)
	}

	if c.Output.TypeScript == "" && c.Output.Go == "" {
		return fmt.Errorf("at least one output language must be configured")
	}

	if !c.Schemas.All && len(c.Schemas.Entries) == 0 {
		return fmt.Errorf("no schemas configured (use 'schemas: all' or list specific schemas)")
	}

	for i, schema := range c.Schemas.Entries {
		if schema.Name == "" {
			return fmt.Errorf("schema at index %d has no name", i)
		}
	}

	return nil
}

func (c *Config) applyDefaults() {
	// TypeScript defaults
	if c.Options.TypeScript.Exports == "" {
		c.Options.TypeScript.Exports = "named"
	}

	// Go defaults
	if c.Options.Go.Package == "" {
		c.Options.Go.Package = "schemas"
	}
	if c.Options.Go.JSONTags == "" {
		c.Options.Go.JSONTags = "omitempty"
	}
}

// GetLanguagesForSchema returns the languages to generate for a schema.
func (c *Config) GetLanguagesForSchema(entry SchemaEntry) []string {
	if len(entry.Languages) > 0 {
		return entry.Languages
	}

	// Default: all configured languages
	var langs []string
	if c.Output.TypeScript != "" {
		langs = append(langs, "typescript")
	}
	if c.Output.Go != "" {
		langs = append(langs, "go")
	}
	return langs
}

// CreateDefaultConfig creates a default .notif.yaml configuration.
func CreateDefaultConfig() *Config {
	return &Config{
		Version: 1,
		Output: OutputConfig{
			TypeScript: "./src/generated/notif",
			Go:         "./internal/notif/schemas",
		},
		Options: OptionsConfig{
			TypeScript: TypeScriptOptions{
				Exports: "named",
			},
			Go: GoOptions{
				Package:  "schemas",
				JSONTags: "omitempty",
			},
		},
		Schemas: SchemaList{
			Entries: []SchemaEntry{
				{Name: "example-schema"},
			},
		},
	}
}

// CreateAllSchemasConfig creates a config that generates all schemas from server.
func CreateAllSchemasConfig() *Config {
	return &Config{
		Version: 1,
		Output: OutputConfig{
			TypeScript: "./src/generated/notif",
			Go:         "./internal/notif/schemas",
		},
		Options: OptionsConfig{
			TypeScript: TypeScriptOptions{
				Exports: "named",
			},
			Go: GoOptions{
				Package:  "schemas",
				JSONTags: "omitempty",
			},
		},
		Schemas: SchemaList{
			All: true,
		},
	}
}

// WriteConfig writes a config to a file.
func WriteConfig(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	header := `# .notif.yaml - Schema codegen configuration
# Generate typed code from notif.sh JSON Schemas
# Run: notif schema generate

`
	return os.WriteFile(path, []byte(header+string(data)), 0644)
}
