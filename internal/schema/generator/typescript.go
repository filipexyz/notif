package generator

import (
	"fmt"
	"strings"

	"github.com/filipexyz/notif/internal/schema"
)

// GenerateTypeScript generates TypeScript interfaces from a schema
func GenerateTypeScript(s *schema.Schema) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("// Generated from schema: @%s\n", s.Name))
	sb.WriteString(fmt.Sprintf("// Version: %s\n", s.Version))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("// %s\n", s.Description))
	}
	sb.WriteString("\n")

	// Generate interface
	interfaceName := toPascalCase(s.Name)
	sb.WriteString(fmt.Sprintf("export interface %s {\n", interfaceName))

	for fieldName, field := range s.Fields {
		// Add field comment if description exists
		if field.Description != "" {
			sb.WriteString(fmt.Sprintf("  /** %s */\n", field.Description))
		}

		// Determine TypeScript type
		tsType := toTypeScriptType(*field)

		// Add optional marker if not required
		optional := ""
		if !field.Required {
			optional = "?"
		}

		sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", fieldName, optional, tsType))
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

func toTypeScriptType(field schema.Field) string {
	switch field.Type {
	case "string":
		if field.Enum != nil && len(field.Enum) > 0 {
			// Generate union type for enum
			values := make([]string, len(field.Enum))
			for i, v := range field.Enum {
				values[i] = fmt.Sprintf("\"%s\"", v)
			}
			return strings.Join(values, " | ")
		}
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		if field.Items != nil {
			if itemField, ok := field.Items.(*schema.Field); ok {
				itemType := toTypeScriptType(*itemField)
				return fmt.Sprintf("%s[]", itemType)
			}
		}
		return "any[]"
	case "object":
		if field.Properties != nil && len(field.Properties) > 0 {
			// Inline object type
			var fields []string
			for propName, propField := range field.Properties {
				optional := ""
				if !propField.Required {
					optional = "?"
				}
				propType := toTypeScriptType(*propField)
				fields = append(fields, fmt.Sprintf("%s%s: %s", propName, optional, propType))
			}
			return fmt.Sprintf("{ %s }", strings.Join(fields, "; "))
		}
		return "Record<string, any>"
	case "datetime", "date", "time":
		return "string" // ISO format string
	case "email", "uri", "uuid":
		return "string"
	default:
		return "any"
	}
}

func toPascalCase(s string) string {
	// Split by hyphen or underscore
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})

	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]))
			result.WriteString(part[1:])
		}
	}

	return result.String()
}
