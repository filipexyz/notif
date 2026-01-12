package generator

import (
	"fmt"
	"strings"

	"github.com/filipexyz/notif/internal/schema"
)

// GenerateRust generates Rust structs with serde from a schema
func GenerateRust(s *schema.Schema) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString("// Generated from schema\n")
	sb.WriteString(fmt.Sprintf("// Name: %s\n", s.Name))
	sb.WriteString(fmt.Sprintf("// Version: %s\n", s.Version))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("// %s\n", s.Description))
	}
	sb.WriteString("\n")

	// Imports
	sb.WriteString("use serde::{Deserialize, Serialize};\n")
	sb.WriteString("\n")

	// Generate struct
	structName := toPascalCase(s.Name)

	// Add doc comment
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("/// %s\n", s.Description))
	}

	// Derive macros
	sb.WriteString("#[derive(Debug, Clone, Serialize, Deserialize)]\n")
	sb.WriteString(fmt.Sprintf("pub struct %s {\n", structName))

	for fieldName, field := range s.Fields {
		// Add doc comment if description exists
		if field.Description != "" {
			sb.WriteString(fmt.Sprintf("    /// %s\n", field.Description))
		}

		// Determine Rust type
		rustType := toRustType(*field)

		// Make Option if not required
		if !field.Required {
			rustType = fmt.Sprintf("Option<%s>", rustType)
		}

		// Add serde attributes for optional fields
		if !field.Required {
			sb.WriteString("    #[serde(skip_serializing_if = \"Option::is_none\")]\n")
		}

		// Convert field name to snake_case
		snakeName := toSnakeCase(fieldName)

		// Add rename attribute if original name differs
		if snakeName != fieldName {
			sb.WriteString(fmt.Sprintf("    #[serde(rename = \"%s\")]\n", fieldName))
		}

		sb.WriteString(fmt.Sprintf("    pub %s: %s,\n", snakeName, rustType))
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

func toRustType(field schema.Field) string {
	switch field.Type {
	case "string":
		return "String"
	case "integer":
		return "i64"
	case "number":
		return "f64"
	case "boolean":
		return "bool"
	case "array":
		if field.Items != nil {
			if itemField, ok := field.Items.(*schema.Field); ok {
				itemType := toRustType(*itemField)
				return fmt.Sprintf("Vec<%s>", itemType)
			}
		}
		return "Vec<serde_json::Value>"
	case "object":
		return "serde_json::Map<String, serde_json::Value>"
	case "datetime", "date-time", "date", "time", "email", "uri", "uuid":
		return "String"
	default:
		return "serde_json::Value"
	}
}

func toSnakeCase(s string) string {
	// Convert camelCase or PascalCase to snake_case
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
