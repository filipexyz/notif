package generator

import (
	"fmt"
	"strings"

	"github.com/filipexyz/notif/internal/schema"
)

// GeneratePython generates Pydantic models from a schema
func GeneratePython(s *schema.Schema) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString("# Generated from schema\n")
	sb.WriteString(fmt.Sprintf("# Name: %s\n", s.Name))
	sb.WriteString(fmt.Sprintf("# Version: %s\n", s.Version))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("# %s\n", s.Description))
	}
	sb.WriteString("\n")

	// Imports
	imports := collectPythonImports(s)
	sb.WriteString("from pydantic import BaseModel")
	if len(imports) > 0 {
		sb.WriteString(fmt.Sprintf(", %s", strings.Join(imports, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString("from typing import Optional, List, Dict, Any\n")
	sb.WriteString("\n\n")

	// Generate class
	className := toPascalCase(s.Name)
	sb.WriteString(fmt.Sprintf("class %s(BaseModel):\n", className))

	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n\n", s.Description))
	}

	if len(s.Fields) == 0 {
		sb.WriteString("    pass\n")
	} else {
		for fieldName, field := range s.Fields {
			pyType := toPythonType(*field)

			// Make optional if not required
			if !field.Required {
				pyType = fmt.Sprintf("Optional[%s]", pyType)
			}

			// Add default if present
			defaultVal := " = None"
			if field.Default != nil {
				defaultVal = fmt.Sprintf(" = %s", formatPythonDefault(field.Default))
			} else if field.Required {
				defaultVal = "" // Required fields have no default
			}

			// Add field with description as comment
			if field.Description != "" {
				sb.WriteString(fmt.Sprintf("    %s: %s%s  # %s\n", fieldName, pyType, defaultVal, field.Description))
			} else {
				sb.WriteString(fmt.Sprintf("    %s: %s%s\n", fieldName, pyType, defaultVal))
			}
		}
	}

	return sb.String(), nil
}

func collectPythonImports(s *schema.Schema) []string {
	imports := make(map[string]bool)

	for _, field := range s.Fields {
		if field.Format == "email" {
			imports["EmailStr"] = true
		}
		if field.Format == "uri" {
			imports["HttpUrl"] = true
		}
		if field.Type == "datetime" || field.Format == "date-time" {
			// datetime is from standard library
		}
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}

	return result
}

func toPythonType(field schema.Field) string {
	switch field.Type {
	case "string":
		if field.Format == "email" {
			return "EmailStr"
		}
		if field.Format == "uri" {
			return "HttpUrl"
		}
		if field.Enum != nil && len(field.Enum) > 0 {
			// Use Literal for enums
			values := make([]string, len(field.Enum))
			for i, v := range field.Enum {
				values[i] = fmt.Sprintf("\"%s\"", v)
			}
			return fmt.Sprintf("Literal[%s]", strings.Join(values, ", "))
		}
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "array":
		if field.Items != nil {
			if itemField, ok := field.Items.(*schema.Field); ok {
				itemType := toPythonType(*itemField)
				return fmt.Sprintf("List[%s]", itemType)
			}
		}
		return "List[Any]"
	case "object":
		if field.Properties != nil && len(field.Properties) > 0 {
			// For nested objects, use Dict (simplified)
			return "Dict[str, Any]"
		}
		return "Dict[str, Any]"
	case "datetime", "date", "time":
		return "str" // ISO format string
	default:
		return "Any"
	}
}

func formatPythonDefault(val interface{}) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case nil:
		return "None"
	default:
		return fmt.Sprintf("%v", v)
	}
}
