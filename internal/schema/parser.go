package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse parses a schema file (YAML or JSON) and returns a Schema.
func Parse(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	// Auto-detect format based on extension or content
	ext := strings.ToLower(filepath.Ext(path))
	isJSON := ext == ".json" || (ext != ".yaml" && ext != ".yml" && strings.HasPrefix(strings.TrimSpace(string(data)), "{"))

	var schema Schema
	if isJSON {
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	}

	return &schema, nil
}

// ParseBytes parses schema data from bytes.
func ParseBytes(data []byte, format string) (*Schema, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	var schema Schema
	switch strings.ToLower(format) {
	case "json":
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s (use 'json' or 'yaml')", format)
	}

	return &schema, nil
}

// MarshalYAML marshals a schema to YAML.
func MarshalYAML(schema *Schema) ([]byte, error) {
	return yaml.Marshal(schema)
}

// MarshalJSON marshals a schema to JSON.
func MarshalJSON(schema *Schema) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}
