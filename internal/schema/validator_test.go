package schema

import (
	"encoding/json"
	"testing"
)

func TestValidator_Validate(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name       string
		schema     string
		data       string
		wantValid  bool
		wantErrors int
	}{
		{
			name: "valid object with required fields",
			schema: `{
				"type": "object",
				"required": ["name", "email"],
				"properties": {
					"name": {"type": "string"},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			data:      `{"name": "John", "email": "john@example.com"}`,
			wantValid: true,
		},
		{
			name: "missing required field",
			schema: `{
				"type": "object",
				"required": ["name", "email"],
				"properties": {
					"name": {"type": "string"},
					"email": {"type": "string"}
				}
			}`,
			data:       `{"name": "John"}`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "wrong type",
			schema: `{
				"type": "object",
				"properties": {
					"age": {"type": "integer"}
				}
			}`,
			data:       `{"age": "twenty"}`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "valid array",
			schema: `{
				"type": "array",
				"items": {"type": "string"}
			}`,
			data:      `["one", "two", "three"]`,
			wantValid: true,
		},
		{
			name: "invalid array item",
			schema: `{
				"type": "array",
				"items": {"type": "string"}
			}`,
			data:       `["one", 2, "three"]`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "nested object validation",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"required": ["id"],
						"properties": {
							"id": {"type": "integer"},
							"name": {"type": "string"}
						}
					}
				}
			}`,
			data:      `{"user": {"id": 123, "name": "John"}}`,
			wantValid: true,
		},
		{
			name: "nested object missing required",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"required": ["id"],
						"properties": {
							"id": {"type": "integer"},
							"name": {"type": "string"}
						}
					}
				}
			}`,
			data:       `{"user": {"name": "John"}}`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "enum validation pass",
			schema: `{
				"type": "object",
				"properties": {
					"status": {"type": "string", "enum": ["pending", "active", "completed"]}
				}
			}`,
			data:      `{"status": "active"}`,
			wantValid: true,
		},
		{
			name: "enum validation fail",
			schema: `{
				"type": "object",
				"properties": {
					"status": {"type": "string", "enum": ["pending", "active", "completed"]}
				}
			}`,
			data:       `{"status": "unknown"}`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "additional properties false",
			schema: `{
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"name": {"type": "string"}
				}
			}`,
			data:       `{"name": "John", "extra": "field"}`,
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "minimum and maximum",
			schema: `{
				"type": "object",
				"properties": {
					"age": {"type": "integer", "minimum": 0, "maximum": 150}
				}
			}`,
			data:      `{"age": 25}`,
			wantValid: true,
		},
		{
			name: "value below minimum",
			schema: `{
				"type": "object",
				"properties": {
					"age": {"type": "integer", "minimum": 0}
				}
			}`,
			data:       `{"age": -5}`,
			wantValid:  false,
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.Validate(json.RawMessage(tt.schema), json.RawMessage(tt.data))
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if tt.wantErrors > 0 && len(result.Errors) < tt.wantErrors {
				t.Errorf("Validate() errors = %d, want at least %d", len(result.Errors), tt.wantErrors)
			}
		})
	}
}

func TestValidator_ValidateWithVersion(t *testing.T) {
	v := NewValidator()

	sv := &SchemaVersion{
		ID:       "schv_test",
		SchemaID: "sch_test",
		Version:  "1.0.0",
		SchemaJSON: json.RawMessage(`{
			"type": "object",
			"required": ["orderId"],
			"properties": {
				"orderId": {"type": "string"},
				"amount": {"type": "number"}
			}
		}`),
		ValidationMode: ValidationModeStrict,
	}

	t.Run("valid data includes version info", func(t *testing.T) {
		data := json.RawMessage(`{"orderId": "ord_123", "amount": 99.99}`)
		result, err := v.ValidateWithVersion(sv, data)
		if err != nil {
			t.Fatalf("ValidateWithVersion() error = %v", err)
		}

		if !result.Valid {
			t.Error("ValidateWithVersion() should be valid")
		}
		if result.Version != "1.0.0" {
			t.Errorf("ValidateWithVersion() version = %q, want %q", result.Version, "1.0.0")
		}
	})

	t.Run("invalid data includes version info", func(t *testing.T) {
		data := json.RawMessage(`{"amount": 99.99}`)
		result, err := v.ValidateWithVersion(sv, data)
		if err != nil {
			t.Fatalf("ValidateWithVersion() error = %v", err)
		}

		if result.Valid {
			t.Error("ValidateWithVersion() should be invalid")
		}
		if result.Version != "1.0.0" {
			t.Errorf("ValidateWithVersion() version = %q, want %q", result.Version, "1.0.0")
		}
	})
}

func TestFingerprint(t *testing.T) {
	tests := []struct {
		name    string
		schema1 string
		schema2 string
		same    bool
	}{
		{
			name:    "identical schemas same fingerprint",
			schema1: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
			schema2: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
			same:    true,
		},
		{
			name:    "different key order same fingerprint",
			schema1: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
			schema2: `{"properties": {"name": {"type": "string"}}, "type": "object"}`,
			same:    true,
		},
		{
			name:    "different schemas different fingerprint",
			schema1: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
			schema2: `{"type": "object", "properties": {"email": {"type": "string"}}}`,
			same:    false,
		},
		{
			name:    "whitespace ignored",
			schema1: `{"type":"object"}`,
			schema2: `{  "type" :  "object"  }`,
			same:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp1 := Fingerprint(json.RawMessage(tt.schema1))
			fp2 := Fingerprint(json.RawMessage(tt.schema2))

			if tt.same && fp1 != fp2 {
				t.Errorf("Fingerprint() should be same for %q and %q", tt.schema1, tt.schema2)
			}
			if !tt.same && fp1 == fp2 {
				t.Errorf("Fingerprint() should be different for %q and %q", tt.schema1, tt.schema2)
			}
		})
	}
}

func TestIsValidSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		{
			name:    "valid simple schema",
			schema:  `{"type": "object"}`,
			wantErr: false,
		},
		{
			name: "valid complex schema",
			schema: `{
				"type": "object",
				"required": ["name"],
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer", "minimum": 0}
				}
			}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			schema:  `{not valid json}`,
			wantErr: true,
		},
		{
			name:    "invalid type value",
			schema:  `{"type": "invalid_type"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidSchema(json.RawMessage(tt.schema))
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_Caching(t *testing.T) {
	v := NewValidator()

	schema := json.RawMessage(`{"type": "object", "properties": {"name": {"type": "string"}}}`)
	data := json.RawMessage(`{"name": "test"}`)

	// First validation - should compile and cache
	result1, err := v.Validate(schema, data)
	if err != nil {
		t.Fatalf("First Validate() error = %v", err)
	}

	// Second validation - should use cached schema
	result2, err := v.Validate(schema, data)
	if err != nil {
		t.Fatalf("Second Validate() error = %v", err)
	}

	if result1.Valid != result2.Valid {
		t.Error("Cached validation should produce same result")
	}
}
