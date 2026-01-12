package schema

import (
	"testing"
)

func TestConvert_SimpleTypes(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"str_field":  {Type: "string"},
			"int_field":  {Type: "integer"},
			"bool_field": {Type: "boolean"},
			"num_field":  {Type: "number"},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if js.Properties["str_field"].(map[string]any)["type"] != "string" {
		t.Error("str_field type mismatch")
	}
	if js.Properties["int_field"].(map[string]any)["type"] != "integer" {
		t.Error("int_field type mismatch")
	}
	if js.Properties["bool_field"].(map[string]any)["type"] != "boolean" {
		t.Error("bool_field type mismatch")
	}
	if js.Properties["num_field"].(map[string]any)["type"] != "number" {
		t.Error("num_field type mismatch")
	}
}

func TestConvert_ArrayType(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"tags": {
				Type:  "array",
				Items: "string",
			},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["tags"].(map[string]any)
	if prop["type"] != "array" {
		t.Error("tags type should be array")
	}

	items := prop["items"].(map[string]any)
	if items["type"] != "string" {
		t.Error("tags items type should be string")
	}
}

func TestConvert_ObjectType(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"metadata": {
				Type: "object",
				Properties: map[string]*Field{
					"key": {Type: "string", Required: true},
				},
			},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["metadata"].(map[string]any)
	if prop["type"] != "object" {
		t.Error("metadata type should be object")
	}

	props := prop["properties"].(map[string]any)
	if props["key"].(map[string]any)["type"] != "string" {
		t.Error("metadata.key type should be string")
	}

	required := prop["required"].([]string)
	if len(required) != 1 || required[0] != "key" {
		t.Error("metadata should have required=['key']")
	}
}

func TestConvert_EnumType(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"status": {
				Type:   "enum",
				Values: []string{"active", "inactive"},
			},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["status"].(map[string]any)
	enum := prop["enum"].([]any)
	if len(enum) != 2 {
		t.Errorf("enum length = %v, want 2", len(enum))
	}
	if enum[0] != "active" || enum[1] != "inactive" {
		t.Error("enum values mismatch")
	}
}

func TestConvert_RequiredFields(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"id":   {Type: "string", Required: true},
			"name": {Type: "string", Required: false},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if len(js.Required) != 1 {
		t.Errorf("required length = %v, want 1", len(js.Required))
	}
	if js.Required[0] != "id" {
		t.Errorf("required[0] = %v, want 'id'", js.Required[0])
	}
}

func TestConvert_OptionalFields(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"name": {Type: "string"},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if js.Required != nil {
		t.Errorf("required should be nil when no required fields, got %v", js.Required)
	}
}

func TestConvert_DefaultValues(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"count": {Type: "integer", Default: 0},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["count"].(map[string]any)
	if prop["default"] != 0 {
		t.Errorf("default = %v, want 0", prop["default"])
	}
}

func TestConvert_Descriptions(t *testing.T) {
	schema := &Schema{
		Name:        "test",
		Version:     "1.0.0",
		Description: "Test schema",
		Fields: map[string]*Field{
			"id": {Type: "string", Description: "Unique ID"},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if js.Description != "Test schema" {
		t.Errorf("description = %v, want 'Test schema'", js.Description)
	}

	prop := js.Properties["id"].(map[string]any)
	if prop["description"] != "Unique ID" {
		t.Errorf("field description = %v, want 'Unique ID'", prop["description"])
	}
}

func TestConvert_MinMax(t *testing.T) {
	min := 0.0
	max := 100.0
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"age": {Type: "integer", Min: &min, Max: &max},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["age"].(map[string]any)
	if prop["minimum"] != 0.0 {
		t.Errorf("minimum = %v, want 0", prop["minimum"])
	}
	if prop["maximum"] != 100.0 {
		t.Errorf("maximum = %v, want 100", prop["maximum"])
	}
}

func TestConvert_DatetimeType(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"created_at": {Type: "datetime"},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	prop := js.Properties["created_at"].(map[string]any)
	if prop["type"] != "string" {
		t.Error("datetime should be string type")
	}
	if prop["format"] != "date-time" {
		t.Error("datetime should have format date-time")
	}
}

func TestConvert_DeepNesting(t *testing.T) {
	schema := &Schema{
		Name:    "test",
		Version: "1.0.0",
		Fields: map[string]*Field{
			"level1": {
				Type: "object",
				Properties: map[string]*Field{
					"level2": {
						Type: "object",
						Properties: map[string]*Field{
							"level3": {
								Type: "object",
								Properties: map[string]*Field{
									"value": {Type: "string"},
								},
							},
						},
					},
				},
			},
		},
	}

	js, err := Convert(schema, "test", "")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	// Navigate nested structure
	l1 := js.Properties["level1"].(map[string]any)
	l1props := l1["properties"].(map[string]any)
	l2 := l1props["level2"].(map[string]any)
	l2props := l2["properties"].(map[string]any)
	l3 := l2props["level3"].(map[string]any)
	l3props := l3["properties"].(map[string]any)
	value := l3props["value"].(map[string]any)

	if value["type"] != "string" {
		t.Error("deeply nested value should be string")
	}
}

func TestParseSchemaRef(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		wantNS    string
		wantName  string
		wantVer   string
		wantError bool
	}{
		{
			name:     "with @ prefix and version",
			ref:      "@filipelabs/agent@1.0.0",
			wantNS:   "filipelabs",
			wantName: "agent",
			wantVer:  "1.0.0",
		},
		{
			name:     "without @ prefix",
			ref:      "filipelabs/agent@1.0.0",
			wantNS:   "filipelabs",
			wantName: "agent",
			wantVer:  "1.0.0",
		},
		{
			name:     "without version",
			ref:      "@filipelabs/agent",
			wantNS:   "filipelabs",
			wantName: "agent",
			wantVer:  "",
		},
		{
			name:      "invalid format - no slash",
			ref:       "@filipelabs",
			wantError: true,
		},
		{
			name:      "invalid format - empty namespace",
			ref:       "@/agent",
			wantError: true,
		},
		{
			name:      "invalid format - empty name",
			ref:       "@filipelabs/",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, ver, err := ParseSchemaRef(tt.ref)
			if tt.wantError {
				if err == nil {
					t.Error("ParseSchemaRef() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseSchemaRef() error = %v", err)
			}

			if ns != tt.wantNS {
				t.Errorf("namespace = %v, want %v", ns, tt.wantNS)
			}
			if name != tt.wantName {
				t.Errorf("name = %v, want %v", name, tt.wantName)
			}
			if ver != tt.wantVer {
				t.Errorf("version = %v, want %v", ver, tt.wantVer)
			}
		})
	}
}

func TestFormatSchemaRef(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		schemaName string
		version   string
		want      string
	}{
		{
			name:       "with version",
			namespace:  "filipelabs",
			schemaName: "agent",
			version:    "1.0.0",
			want:       "@filipelabs/agent@1.0.0",
		},
		{
			name:       "without version",
			namespace:  "filipelabs",
			schemaName: "agent",
			version:    "",
			want:       "@filipelabs/agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSchemaRef(tt.namespace, tt.schemaName, tt.version)
			if got != tt.want {
				t.Errorf("FormatSchemaRef() = %v, want %v", got, tt.want)
			}
		})
	}
}
