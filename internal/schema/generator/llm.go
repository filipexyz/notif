package generator

import (
	"fmt"
	"strings"

	"github.com/filipexyz/notif/internal/schema"
)

// GenerateLLM generates LLM-friendly markdown documentation from a schema
func GenerateLLM(s *schema.Schema, namespace string) (string, error) {
	var sb strings.Builder

	// Title
	sb.WriteString(fmt.Sprintf("# Schema: @%s/%s\n\n", namespace, s.Name))

	// Version
	sb.WriteString(fmt.Sprintf("**Version:** %s\n\n", s.Version))

	// Description
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", s.Description))
	}

	// Overview
	sb.WriteString("## Overview\n\n")
	sb.WriteString(fmt.Sprintf("This schema defines the structure for `%s` events in the notif.sh system.\n", s.Name))
	sb.WriteString("Use this schema to validate and generate type-safe event payloads.\n\n")

	// Fields
	if len(s.Fields) > 0 {
		sb.WriteString("## Fields\n\n")

		for fieldName, field := range s.Fields {
			sb.WriteString(fmt.Sprintf("### `%s`\n\n", fieldName))

			// Type and required
			sb.WriteString(fmt.Sprintf("- **Type:** `%s`\n", field.Type))
			if field.Required {
				sb.WriteString("- **Required:** Yes\n")
			} else {
				sb.WriteString("- **Required:** No\n")
			}

			// Format
			if field.Format != "" {
				sb.WriteString(fmt.Sprintf("- **Format:** `%s`\n", field.Format))
			}

			// Enum values
			if field.Enum != nil && len(field.Enum) > 0 {
				sb.WriteString("- **Allowed values:**\n")
				for _, v := range field.Enum {
					sb.WriteString(fmt.Sprintf("  - `%s`\n", v))
				}
			}

			// Default
			if field.Default != nil {
				sb.WriteString(fmt.Sprintf("- **Default:** `%v`\n", field.Default))
			}

			// Constraints
			if field.MinLength > 0 {
				sb.WriteString(fmt.Sprintf("- **Min length:** %d\n", field.MinLength))
			}
			if field.MaxLength > 0 {
				sb.WriteString(fmt.Sprintf("- **Max length:** %d\n", field.MaxLength))
			}
			if field.Min != nil {
				sb.WriteString(fmt.Sprintf("- **Minimum:** %v\n", *field.Min))
			}
			if field.Max != nil {
				sb.WriteString(fmt.Sprintf("- **Maximum:** %v\n", *field.Max))
			}
			if field.MinItems > 0 {
				sb.WriteString(fmt.Sprintf("- **Min items:** %d\n", field.MinItems))
			}
			if field.MaxItems > 0 {
				sb.WriteString(fmt.Sprintf("- **Max items:** %d\n", field.MaxItems))
			}

			// Description
			if field.Description != "" {
				sb.WriteString(fmt.Sprintf("\n%s\n", field.Description))
			}

			sb.WriteString("\n")
		}
	}

	// Example
	sb.WriteString("## Example Payload\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(generateExampleJSON(s))
	sb.WriteString("\n```\n\n")

	// Usage instructions
	sb.WriteString("## Usage\n\n")
	sb.WriteString("### Installation\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("notif schema install @%s/%s\n", namespace, s.Name))
	sb.WriteString("```\n\n")

	sb.WriteString("### Validation\n\n")
	sb.WriteString("This schema can be used to validate event payloads before publishing:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Bind schema to topic\n")
	sb.WriteString(fmt.Sprintf("notif topic bind 'your.topic' @%s/%s\n\n", namespace, s.Name))
	sb.WriteString("# Emit will now validate against schema\n")
	sb.WriteString("notif emit your.topic '{\"your\":\"data\"}'\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### Code Generation\n\n")
	sb.WriteString("Generate type-safe interfaces for your favorite language:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# TypeScript\n")
	sb.WriteString(fmt.Sprintf("notif schema export @%s/%s --format=typescript\n\n", namespace, s.Name))
	sb.WriteString("# Python (Pydantic)\n")
	sb.WriteString(fmt.Sprintf("notif schema export @%s/%s --format=python\n\n", namespace, s.Name))
	sb.WriteString("# Go\n")
	sb.WriteString(fmt.Sprintf("notif schema export @%s/%s --format=go\n\n", namespace, s.Name))
	sb.WriteString("# Rust\n")
	sb.WriteString(fmt.Sprintf("notif schema export @%s/%s --format=rust\n", namespace, s.Name))
	sb.WriteString("```\n\n")

	// Schema reference
	sb.WriteString("## Schema Reference\n\n")
	sb.WriteString(fmt.Sprintf("- **Namespace:** `%s`\n", namespace))
	sb.WriteString(fmt.Sprintf("- **Name:** `%s`\n", s.Name))
	sb.WriteString(fmt.Sprintf("- **Version:** `%s`\n", s.Version))
	sb.WriteString(fmt.Sprintf("- **Full reference:** `@%s/%s@%s`\n", namespace, s.Name, s.Version))

	return sb.String(), nil
}

func generateExampleJSON(s *schema.Schema) string {
	lines := []string{"{"}

	i := 0
	total := len(s.Fields)
	for fieldName, field := range s.Fields {
		i++
		comma := ","
		if i == total {
			comma = ""
		}

		value := getExampleValueString(*field)
		comment := ""
		if field.Description != "" {
			comment = fmt.Sprintf("  // %s", field.Description)
		}

		lines = append(lines, fmt.Sprintf("  \"%s\": %s%s%s", fieldName, value, comma, comment))
	}

	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func getExampleValueString(field schema.Field) string {
	switch field.Type {
	case "string":
		if field.Default != nil {
			if s, ok := field.Default.(string); ok {
				return fmt.Sprintf("\"%s\"", s)
			}
		}
		if field.Enum != nil && len(field.Enum) > 0 {
			return fmt.Sprintf("\"%s\"", field.Enum[0])
		}
		if field.Format == "email" {
			return "\"user@example.com\""
		}
		if field.Format == "uri" {
			return "\"https://example.com\""
		}
		if field.Format == "uuid" {
			return "\"123e4567-e89b-12d3-a456-426614174000\""
		}
		if field.Format == "date" {
			return "\"2024-01-15\""
		}
		if field.Format == "time" {
			return "\"14:30:00\""
		}
		if field.Format == "date-time" || field.Type == "datetime" {
			return "\"2024-01-15T14:30:00Z\""
		}
		return "\"example\""
	case "integer":
		if field.Default != nil {
			return fmt.Sprintf("%v", field.Default)
		}
		return "42"
	case "number":
		if field.Default != nil {
			return fmt.Sprintf("%v", field.Default)
		}
		return "3.14"
	case "boolean":
		if field.Default != nil {
			if b, ok := field.Default.(bool); ok {
				if b {
					return "true"
				}
				return "false"
			}
		}
		return "true"
	case "array":
		return "[\"item1\", \"item2\"]"
	case "object":
		return "{}"
	default:
		return "null"
	}
}
