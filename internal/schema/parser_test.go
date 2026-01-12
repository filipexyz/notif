package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAML_ValidSchema(t *testing.T) {
	content := `name: test-schema
version: 1.0.0
description: Test schema
fields:
  id:
    type: string
    required: true
`
	tmpFile := createTempFile(t, "schema.yaml", content)
	defer os.Remove(tmpFile)

	schema, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if schema.Name != "test-schema" {
		t.Errorf("Name = %v, want %v", schema.Name, "test-schema")
	}
	if schema.Version != "1.0.0" {
		t.Errorf("Version = %v, want %v", schema.Version, "1.0.0")
	}
	if len(schema.Fields) != 1 {
		t.Errorf("len(Fields) = %v, want %v", len(schema.Fields), 1)
	}
}

func TestParseYAML_InvalidSyntax(t *testing.T) {
	content := `name: test
version: 1.0.0
fields:
  id:
    type: string
	invalid indentation
`
	tmpFile := createTempFile(t, "schema.yaml", content)
	defer os.Remove(tmpFile)

	_, err := Parse(tmpFile)
	if err == nil {
		t.Error("Parse() expected error for invalid YAML, got nil")
	}
}

func TestParseYAML_MissingName(t *testing.T) {
	content := `version: 1.0.0
fields:
  id:
    type: string
`
	tmpFile := createTempFile(t, "schema.yaml", content)
	defer os.Remove(tmpFile)

	schema, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Parse should succeed, validation will catch missing name
	if schema.Name != "" {
		t.Errorf("Name = %v, want empty", schema.Name)
	}
}

func TestParseYAML_MissingVersion(t *testing.T) {
	content := `name: test-schema
fields:
  id:
    type: string
`
	tmpFile := createTempFile(t, "schema.yaml", content)
	defer os.Remove(tmpFile)

	schema, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Parse should succeed, validation will catch missing version
	if schema.Version != "" {
		t.Errorf("Version = %v, want empty", schema.Version)
	}
}

func TestParseJSON_ValidSchema(t *testing.T) {
	content := `{
  "name": "test-schema",
  "version": "1.0.0",
  "fields": {
    "id": {
      "type": "string",
      "required": true
    }
  }
}`
	tmpFile := createTempFile(t, "schema.json", content)
	defer os.Remove(tmpFile)

	schema, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if schema.Name != "test-schema" {
		t.Errorf("Name = %v, want %v", schema.Name, "test-schema")
	}
}

func TestParseJSON_InvalidSyntax(t *testing.T) {
	content := `{
  "name": "test",
  "version": "1.0.0",
  invalid json
}`
	tmpFile := createTempFile(t, "schema.json", content)
	defer os.Remove(tmpFile)

	_, err := Parse(tmpFile)
	if err == nil {
		t.Error("Parse() expected error for invalid JSON, got nil")
	}
}

func TestParse_DetectsFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
	}{
		{
			name:     "YAML by extension",
			filename: "schema.yaml",
			content:  "name: test\nversion: 1.0.0\nfields: {}",
		},
		{
			name:     "JSON by extension",
			filename: "schema.json",
			content:  `{"name":"test","version":"1.0.0","fields":{}}`,
		},
		{
			name:     "JSON by content",
			filename: "schema",
			content:  `{"name":"test","version":"1.0.0","fields":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.filename, tt.content)
			defer os.Remove(tmpFile)

			schema, err := Parse(tmpFile)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if schema.Name != "test" {
				t.Errorf("Name = %v, want %v", schema.Name, "test")
			}
		})
	}
}

func TestParse_EmptyFile(t *testing.T) {
	tmpFile := createTempFile(t, "schema.yaml", "")
	defer os.Remove(tmpFile)

	_, err := Parse(tmpFile)
	if err == nil {
		t.Error("Parse() expected error for empty file, got nil")
	}
}

func TestParse_LargeSchema(t *testing.T) {
	// Generate schema with 100 fields
	content := "name: large-schema\nversion: 1.0.0\nfields:\n"
	for i := 0; i < 100; i++ {
		content += "  field_" + fmt.Sprintf("%03d", i) + ":\n"
		content += "    type: string\n"
	}

	tmpFile := createTempFile(t, "schema.yaml", content)
	defer os.Remove(tmpFile)

	schema, err := Parse(tmpFile)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(schema.Fields) != 100 {
		t.Errorf("len(Fields) = %v, want %v", len(schema.Fields), 100)
	}
}

func TestParseBytes_ValidYAML(t *testing.T) {
	content := []byte("name: test\nversion: 1.0.0\nfields: {}")

	schema, err := ParseBytes(content, "yaml")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if schema.Name != "test" {
		t.Errorf("Name = %v, want %v", schema.Name, "test")
	}
}

func TestParseBytes_ValidJSON(t *testing.T) {
	content := []byte(`{"name":"test","version":"1.0.0","fields":{}}`)

	schema, err := ParseBytes(content, "json")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if schema.Name != "test" {
		t.Errorf("Name = %v, want %v", schema.Name, "test")
	}
}

func TestParseBytes_EmptyData(t *testing.T) {
	_, err := ParseBytes([]byte{}, "yaml")
	if err == nil {
		t.Error("ParseBytes() expected error for empty data, got nil")
	}
}

func TestParseBytes_UnsupportedFormat(t *testing.T) {
	content := []byte("test")
	_, err := ParseBytes(content, "xml")
	if err == nil {
		t.Error("ParseBytes() expected error for unsupported format, got nil")
	}
}

func TestMarshalYAML(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"id": {Type: "string", Required: true},
		},
	}

	data, err := MarshalYAML(schema)
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	// Should be valid YAML
	parsed, err := ParseBytes(data, "yaml")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if parsed.Name != "test" {
		t.Errorf("Name = %v, want %v", parsed.Name, "test")
	}
}

func TestMarshalJSON(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"id": {Type: "string", Required: true},
		},
	}

	data, err := MarshalJSON(schema)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Should be valid JSON
	parsed, err := ParseBytes(data, "json")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if parsed.Name != "test" {
		t.Errorf("Name = %v, want %v", parsed.Name, "test")
	}
}

// Helper function to create temporary files for testing
func createTempFile(t *testing.T, name, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, name)

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpFile
}
