package schema

import (
	"encoding/json"
	"time"
)

// ValidationMode determines how validation failures are handled.
type ValidationMode string

const (
	ValidationModeStrict   ValidationMode = "strict"   // Reject invalid events
	ValidationModeWarn     ValidationMode = "warn"     // Log but allow
	ValidationModeDisabled ValidationMode = "disabled" // Skip validation
)

// OnInvalid determines what happens when validation fails in strict mode.
type OnInvalid string

const (
	OnInvalidReject OnInvalid = "reject" // Return error to client
	OnInvalidLog    OnInvalid = "log"    // Log and continue
	OnInvalidDLQ    OnInvalid = "dlq"    // Send to dead letter queue
)

// Compatibility determines how schema evolution is checked.
type Compatibility string

const (
	CompatibilityBackward Compatibility = "backward" // New can read old
	CompatibilityForward  Compatibility = "forward"  // Old can read new
	CompatibilityFull     Compatibility = "full"     // Both directions
	CompatibilityNone     Compatibility = "none"     // No checking
)

// Schema represents a schema definition.
type Schema struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	ProjectID    string    `json:"project_id"`
	Name         string    `json:"name"`
	TopicPattern string    `json:"topic_pattern"`
	Description  string    `json:"description,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Loaded from latest version
	LatestVersion *SchemaVersion `json:"latest_version,omitempty"`
}

// SchemaVersion represents an immutable version of a schema.
type SchemaVersion struct {
	ID             string          `json:"id"`
	SchemaID       string          `json:"schema_id"`
	Version        string          `json:"version"`
	SchemaJSON     json.RawMessage `json:"schema"`
	ValidationMode ValidationMode  `json:"validation_mode"`
	OnInvalid      OnInvalid       `json:"on_invalid"`
	Compatibility  Compatibility   `json:"compatibility"`
	Examples       json.RawMessage `json:"examples,omitempty"`
	Fingerprint    string          `json:"fingerprint"`
	IsLatest       bool            `json:"is_latest"`
	CreatedAt      time.Time       `json:"created_at"`
	CreatedBy      string          `json:"created_by,omitempty"`
}

// SchemaValidation represents a validation result log entry.
type SchemaValidation struct {
	ID              string          `json:"id"`
	OrgID           string          `json:"org_id"`
	ProjectID       string          `json:"project_id"`
	EventID         string          `json:"event_id,omitempty"`
	SchemaID        string          `json:"schema_id,omitempty"`
	SchemaVersionID string          `json:"schema_version_id,omitempty"`
	Topic           string          `json:"topic"`
	Valid           bool            `json:"valid"`
	Errors          json.RawMessage `json:"errors,omitempty"`
	ValidatedAt     time.Time       `json:"validated_at"`
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// ValidationResult is the outcome of validating data against a schema.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
	Schema  string            `json:"schema,omitempty"`
	Version string            `json:"version,omitempty"`
}

// SchemaDefinition represents the YAML schema file structure.
type SchemaDefinition struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Topic       string   `yaml:"topic" json:"topic"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	Schema json.RawMessage `yaml:"schema" json:"schema"`

	Validation *ValidationConfig `yaml:"validation,omitempty" json:"validation,omitempty"`

	Compatibility Compatibility `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`

	Examples []json.RawMessage `yaml:"examples,omitempty" json:"examples,omitempty"`
}

// ValidationConfig holds validation settings.
type ValidationConfig struct {
	Mode      ValidationMode `yaml:"mode" json:"mode"`
	OnInvalid OnInvalid      `yaml:"onInvalid" json:"on_invalid"`
}

// CreateSchemaRequest is the API request to create a schema.
type CreateSchemaRequest struct {
	Name         string   `json:"name"`
	TopicPattern string   `json:"topic_pattern"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// CreateSchemaVersionRequest is the API request to create a schema version.
type CreateSchemaVersionRequest struct {
	Version        string          `json:"version"`
	Schema         json.RawMessage `json:"schema"`
	ValidationMode ValidationMode  `json:"validation_mode,omitempty"`
	OnInvalid      OnInvalid       `json:"on_invalid,omitempty"`
	Compatibility  Compatibility   `json:"compatibility,omitempty"`
	Examples       json.RawMessage `json:"examples,omitempty"`
}

// UpdateSchemaRequest is the API request to update a schema.
type UpdateSchemaRequest struct {
	TopicPattern string   `json:"topic_pattern,omitempty"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ValidateRequest is the API request to validate data.
type ValidateRequest struct {
	Data json.RawMessage `json:"data"`
}

// ValidationStats holds validation statistics.
type ValidationStats struct {
	Total        int64 `json:"total"`
	ValidCount   int64 `json:"valid_count"`
	InvalidCount int64 `json:"invalid_count"`
}
