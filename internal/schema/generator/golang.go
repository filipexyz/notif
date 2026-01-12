package generator

import (
	"fmt"
	"strings"

	"github.com/filipexyz/notif/internal/schema"
)

// GenerateGo generates Go structs from a schema
func GenerateGo(s *schema.Schema, packageName string) (string, error) {
	if packageName == "" {
		packageName = "main"
	}

	var sb strings.Builder

	// Package declaration
	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	// Imports (if needed)
	needsTime := false
	for _, field := range s.Fields {
		if field.Type == "datetime" || field.Format == "date-time" {
			needsTime = true
			break
		}
	}

	if needsTime {
		sb.WriteString("import \"time\"\n\n")
	}

	// Header comment
	sb.WriteString(fmt.Sprintf("// %s represents the %s schema\n", toPascalCase(s.Name), s.Name))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("// %s\n", s.Description))
	}
	sb.WriteString(fmt.Sprintf("// Version: %s\n", s.Version))

	// Generate struct
	structName := toPascalCase(s.Name)
	sb.WriteString(fmt.Sprintf("type %s struct {\n", structName))

	for fieldName, field := range s.Fields {
		// Add field comment if description exists
		if field.Description != "" {
			sb.WriteString(fmt.Sprintf("\t// %s\n", field.Description))
		}

		// Determine Go type
		goType := toGoType(*field)

		// Make pointer if not required
		if !field.Required {
			goType = "*" + goType
		}

		// Format field name (PascalCase)
		goFieldName := toPascalCase(fieldName)

		// JSON tag
		jsonTag := fieldName
		if !field.Required {
			jsonTag += ",omitempty"
		}

		sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", goFieldName, goType, jsonTag))
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

func toGoType(field schema.Field) string {
	switch field.Type {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		if field.Items != nil {
			if itemField, ok := field.Items.(*schema.Field); ok {
				itemType := toGoType(*itemField)
				return fmt.Sprintf("[]%s", itemType)
			}
		}
		return "[]interface{}"
	case "object":
		return "map[string]interface{}"
	case "datetime", "date-time":
		return "time.Time"
	case "date", "time", "email", "uri", "uuid":
		return "string"
	default:
		return "interface{}"
	}
}
