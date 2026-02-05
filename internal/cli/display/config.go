package display

import "encoding/json"

// DisplayConfig holds the display configuration for events.
type DisplayConfig struct {
	TopicPattern string            `json:"topicPattern,omitempty" yaml:"topicPattern,omitempty"`
	Template     string            `json:"template,omitempty" yaml:"template,omitempty"`
	Fields       []FieldConfig     `json:"fields,omitempty" yaml:"fields,omitempty"`
	Conditions   []ConditionConfig `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// FieldConfig configures a single field in the display output.
type FieldConfig struct {
	Path       string            `json:"path" yaml:"path"`
	Label      string            `json:"label,omitempty" yaml:"label,omitempty"`
	Color      string            `json:"color,omitempty" yaml:"color,omitempty"`
	Format     string            `json:"format,omitempty" yaml:"format,omitempty"`
	Width      int               `json:"width,omitempty" yaml:"width,omitempty"`
	Conditions []ConditionConfig `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// ConditionConfig configures conditional formatting.
type ConditionConfig struct {
	When   string            `json:"when" yaml:"when"`
	Color  string            `json:"color,omitempty" yaml:"color,omitempty"`
	Prefix string            `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Suffix string            `json:"suffix,omitempty" yaml:"suffix,omitempty"`
	Set    map[string]string `json:"set,omitempty" yaml:"set,omitempty"`
}

// ProjectConfig represents the .notif.json file structure.
type ProjectConfig struct {
	Display *ProjectDisplayConfig `json:"display,omitempty" yaml:"display,omitempty"`
}

// ProjectDisplayConfig holds display settings per topic pattern.
type ProjectDisplayConfig struct {
	Topics map[string]*DisplayConfig `json:"topics,omitempty" yaml:"topics,omitempty"`
}

// EventData represents the data available in templates.
type EventData struct {
	ID        string                 `json:"id"`
	Topic     interface{}            `json:"topic"` // string or map with extracted parts
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// SchemaWithDisplay represents a schema with its display configuration.
type SchemaWithDisplay struct {
	Name         string          `json:"name"`
	TopicPattern string          `json:"topic_pattern"`
	Display      *DisplayConfig  `json:"display,omitempty"`
	Schema       json.RawMessage `json:"schema,omitempty"`
}

// CacheIndex stores metadata about the local schema cache.
type CacheIndex struct {
	Server      string `json:"server"`
	LastSync    string `json:"lastSync"`
	ETag        string `json:"etag,omitempty"`
	TTL         int    `json:"ttl"` // seconds
	SchemaCount int    `json:"schemaCount"`
}

// SchemasCache stores cached schemas with display configs.
type SchemasCache struct {
	Schemas []SchemaWithDisplay `json:"schemas"`
}
