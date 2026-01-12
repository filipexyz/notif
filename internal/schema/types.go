package schema

import "time"

// Schema represents a notif.sh schema definition (YAML shorthand format).
type Schema struct {
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version" json:"version"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Fields      map[string]*Field `yaml:"fields" json:"fields"`
}

// Field represents a schema field.
type Field struct {
	Type        string            `yaml:"type" json:"type"`
	Required    bool              `yaml:"required,omitempty" json:"required,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Default     any               `yaml:"default,omitempty" json:"default,omitempty"`
	Format      string            `yaml:"format,omitempty" json:"format,omitempty"`
	Items       any               `yaml:"items,omitempty" json:"items,omitempty"` // For array type: string or *Field
	Properties  map[string]*Field `yaml:"properties,omitempty" json:"properties,omitempty"`
	Enum        []string          `yaml:"enum,omitempty" json:"enum,omitempty"` // For enum values
	Values      []string          `yaml:"values,omitempty" json:"values,omitempty"` // For enum type (alias)
	MinItems    int               `yaml:"minItems,omitempty" json:"minItems,omitempty"`
	MaxItems    int               `yaml:"maxItems,omitempty" json:"maxItems,omitempty"`
	Min         *float64          `yaml:"min,omitempty" json:"min,omitempty"`
	Max         *float64          `yaml:"max,omitempty" json:"max,omitempty"`
	MinLength   int               `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength   int               `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Pattern     string            `yaml:"pattern,omitempty" json:"pattern,omitempty"`
}

// JSONSchema represents a JSON Schema document.
type JSONSchema struct {
	Schema      string                 `json:"$schema"`
	ID          string                 `json:"$id,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"`
	Required    []string               `json:"required,omitempty"`
	Properties  map[string]any         `json:"properties,omitempty"`
	Items       any                    `json:"items,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
	Default     any                    `json:"default,omitempty"`
	MinItems    *int                   `json:"minItems,omitempty"`
	MaxItems    *int                   `json:"maxItems,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Additional  map[string]any         `json:"-"`
}

// InstalledSchema represents a locally installed schema.
type InstalledSchema struct {
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	Path        string    `json:"path"`
}

// ValidationError represents a schema validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// ValidationResult represents the result of schema validation.
type ValidationResult struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []*ValidationError
}

// AddError adds an error to the validation result.
func (r *ValidationResult) AddError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{Field: field, Message: message})
}

// AddWarning adds a warning to the validation result.
func (r *ValidationResult) AddWarning(field, message string) {
	r.Warnings = append(r.Warnings, &ValidationError{Field: field, Message: message})
}
