package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

// Validator validates data against JSON schemas.
type Validator struct {
	cache sync.Map // map[string]*gojsonschema.Schema (fingerprint -> compiled schema)
}

// NewValidator creates a new schema validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates data against a schema.
func (v *Validator) Validate(schemaJSON, data json.RawMessage) (*ValidationResult, error) {
	// Get or compile schema
	compiled, err := v.getCompiledSchema(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	// Load data
	dataLoader := gojsonschema.NewBytesLoader(data)

	// Validate
	result, err := compiled.Validate(dataLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Convert to our result type
	validationResult := &ValidationResult{
		Valid: result.Valid(),
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			validationResult.Errors = append(validationResult.Errors, ValidationError{
				Field:   err.Field(),
				Message: err.Description(),
				Type:    err.Type(),
			})
		}
	}

	return validationResult, nil
}

// ValidateWithVersion validates and includes schema info in the result.
func (v *Validator) ValidateWithVersion(sv *SchemaVersion, data json.RawMessage) (*ValidationResult, error) {
	result, err := v.Validate(sv.SchemaJSON, data)
	if err != nil {
		return nil, err
	}

	result.Version = sv.Version
	return result, nil
}

// getCompiledSchema retrieves a compiled schema from cache or compiles it.
func (v *Validator) getCompiledSchema(schemaJSON json.RawMessage) (*gojsonschema.Schema, error) {
	fingerprint := Fingerprint(schemaJSON)

	// Check cache
	if cached, ok := v.cache.Load(fingerprint); ok {
		return cached.(*gojsonschema.Schema), nil
	}

	// Compile schema
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	compiled, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return nil, err
	}

	// Cache it
	v.cache.Store(fingerprint, compiled)

	return compiled, nil
}

// Fingerprint computes a SHA256 hash of the schema for caching and comparison.
func Fingerprint(schemaJSON json.RawMessage) string {
	// Normalize JSON by re-encoding
	var normalized interface{}
	if err := json.Unmarshal(schemaJSON, &normalized); err != nil {
		// If we can't parse, hash the raw bytes
		hash := sha256.Sum256(schemaJSON)
		return hex.EncodeToString(hash[:])
	}

	// Re-encode with sorted keys
	canonicalized, err := json.Marshal(normalized)
	if err != nil {
		hash := sha256.Sum256(schemaJSON)
		return hex.EncodeToString(hash[:])
	}

	hash := sha256.Sum256(canonicalized)
	return hex.EncodeToString(hash[:])
}

// IsValidSchema checks if a JSON schema is valid.
func IsValidSchema(schemaJSON json.RawMessage) error {
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	_, err := gojsonschema.NewSchema(schemaLoader)
	return err
}
