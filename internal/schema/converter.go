package schema

import (
	"fmt"
	"strings"
)

// Convert converts a Schema (YAML shorthand) to JSONSchema.
func Convert(schema *Schema, namespace, schemaURL string) (*JSONSchema, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	// Build JSON Schema
	js := &JSONSchema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		Title:       schema.Name,
		Description: schema.Description,
		Type:        "object",
		Properties:  make(map[string]any),
		Required:    []string{},
	}

	// Set $id if URL is provided
	if schemaURL != "" {
		js.ID = schemaURL
	} else if namespace != "" {
		js.ID = fmt.Sprintf("https://notif.sh/schemas/%s/%s/%s", namespace, schema.Name, schema.Version)
	}

	// Convert fields
	for name, field := range schema.Fields {
		prop, err := convertField(field, name, make(map[string]bool))
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		js.Properties[name] = prop

		if field.Required {
			js.Required = append(js.Required, name)
		}
	}

	// If no required fields, omit the array
	if len(js.Required) == 0 {
		js.Required = nil
	}

	return js, nil
}

// convertField converts a Field to JSON Schema property.
func convertField(field *Field, fieldName string, visited map[string]bool) (map[string]any, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	// Check for circular references
	if visited[fieldName] {
		return nil, fmt.Errorf("circular reference detected")
	}
	visited[fieldName] = true
	defer delete(visited, fieldName)

	prop := make(map[string]any)

	// Handle type conversion
	switch field.Type {
	case "string":
		prop["type"] = "string"
		if field.MinLength > 0 {
			prop["minLength"] = field.MinLength
		}
		if field.MaxLength > 0 {
			prop["maxLength"] = field.MaxLength
		}
		if field.Pattern != "" {
			prop["pattern"] = field.Pattern
		}

	case "integer":
		prop["type"] = "integer"
		if field.Min != nil {
			prop["minimum"] = *field.Min
		}
		if field.Max != nil {
			prop["maximum"] = *field.Max
		}

	case "number":
		prop["type"] = "number"
		if field.Min != nil {
			prop["minimum"] = *field.Min
		}
		if field.Max != nil {
			prop["maximum"] = *field.Max
		}

	case "boolean":
		prop["type"] = "boolean"

	case "array":
		prop["type"] = "array"
		if field.MinItems > 0 {
			prop["minItems"] = field.MinItems
		}
		if field.MaxItems > 0 {
			prop["maxItems"] = field.MaxItems
		}

		// Handle items
		if field.Items != nil {
			switch items := field.Items.(type) {
			case string:
				// Simple type like "string"
				prop["items"] = map[string]any{"type": items}
			case *Field:
				// Complex nested field
				itemProp, err := convertField(items, fieldName+"[]", visited)
				if err != nil {
					return nil, fmt.Errorf("array items: %w", err)
				}
				prop["items"] = itemProp
			case map[string]any:
				// Already a map (from YAML parsing)
				if nestedField, err := mapToField(items); err == nil {
					itemProp, err := convertField(nestedField, fieldName+"[]", visited)
					if err != nil {
						return nil, fmt.Errorf("array items: %w", err)
					}
					prop["items"] = itemProp
				} else {
					prop["items"] = items
				}
			default:
				return nil, fmt.Errorf("unsupported items type: %T", items)
			}
		}

	case "object":
		prop["type"] = "object"
		if field.Properties != nil && len(field.Properties) > 0 {
			properties := make(map[string]any)
			required := []string{}

			for name, nestedField := range field.Properties {
				nestedProp, err := convertField(nestedField, fieldName+"."+name, visited)
				if err != nil {
					return nil, fmt.Errorf("property %q: %w", name, err)
				}
				properties[name] = nestedProp

				if nestedField.Required {
					required = append(required, name)
				}
			}

			prop["properties"] = properties
			if len(required) > 0 {
				prop["required"] = required
			}
		}

	case "enum":
		if len(field.Values) == 0 {
			return nil, fmt.Errorf("enum must have at least one value")
		}
		// Convert string values to any for enum
		enumValues := make([]any, len(field.Values))
		for i, v := range field.Values {
			enumValues[i] = v
		}
		prop["enum"] = enumValues

	case "datetime":
		// datetime is represented as string with format
		prop["type"] = "string"
		prop["format"] = "date-time"

	case "date":
		prop["type"] = "string"
		prop["format"] = "date"

	case "time":
		prop["type"] = "string"
		prop["format"] = "time"

	case "email":
		prop["type"] = "string"
		prop["format"] = "email"

	case "uri", "url":
		prop["type"] = "string"
		prop["format"] = "uri"

	case "uuid":
		prop["type"] = "string"
		prop["format"] = "uuid"

	default:
		return nil, fmt.Errorf("unsupported type %q (supported: string, integer, number, boolean, array, object, enum, datetime, date, time, email, uri, url, uuid)", field.Type)
	}

	// Add description
	if field.Description != "" {
		prop["description"] = field.Description
	}

	// Add default value
	if field.Default != nil {
		prop["default"] = field.Default
	}

	return prop, nil
}

// mapToField attempts to convert a map[string]any to a Field.
func mapToField(m map[string]any) (*Field, error) {
	field := &Field{}

	if t, ok := m["type"].(string); ok {
		field.Type = t
	} else {
		return nil, fmt.Errorf("missing type")
	}

	if r, ok := m["required"].(bool); ok {
		field.Required = r
	}

	if d, ok := m["description"].(string); ok {
		field.Description = d
	}

	if def, ok := m["default"]; ok {
		field.Default = def
	}

	if items, ok := m["items"]; ok {
		field.Items = items
	}

	if props, ok := m["properties"].(map[string]any); ok {
		field.Properties = make(map[string]*Field)
		for name, prop := range props {
			if propMap, ok := prop.(map[string]any); ok {
				if nestedField, err := mapToField(propMap); err == nil {
					field.Properties[name] = nestedField
				}
			}
		}
	}

	if values, ok := m["values"].([]any); ok {
		field.Values = make([]string, len(values))
		for i, v := range values {
			if s, ok := v.(string); ok {
				field.Values[i] = s
			}
		}
	}

	return field, nil
}

// GenerateTemplate generates a template schema for initialization.
func GenerateTemplate(name string) *Schema {
	if name == "" {
		name = "my-schema"
	}

	return &Schema{
		Name:        name,
		Version:     "1.0.0",
		Description: "Description of your schema",
		Fields: map[string]*Field{
			"id": {
				Type:        "string",
				Required:    true,
				Description: "Unique identifier",
			},
		},
	}
}

// NormalizeNamespace normalizes a namespace (e.g., "@user" to "user").
func NormalizeNamespace(namespace string) string {
	return strings.TrimPrefix(namespace, "@")
}

// parseRefParts parses a schema reference like "@namespace/name@version".
// Returns namespace, name, version.
func parseRefParts(ref string) (namespace, name, version string, err error) {
	// Remove @ prefix if present
	ref = strings.TrimPrefix(ref, "@")

	// Split by @version
	parts := strings.Split(ref, "@")
	if len(parts) > 2 {
		return "", "", "", fmt.Errorf("invalid schema reference format (expected: @namespace/name@version)")
	}

	if len(parts) == 2 {
		version = parts[1]
		ref = parts[0]
	}

	// Split namespace/name
	slashParts := strings.Split(ref, "/")
	if len(slashParts) != 2 {
		return "", "", "", fmt.Errorf("invalid schema reference format (expected: @namespace/name@version)")
	}

	namespace = slashParts[0]
	name = slashParts[1]

	if namespace == "" || name == "" {
		return "", "", "", fmt.Errorf("namespace and name cannot be empty")
	}

	return namespace, name, version, nil
}

// FormatSchemaRef formats a schema reference.
func FormatSchemaRef(namespace, name, version string) string {
	ref := fmt.Sprintf("@%s/%s", namespace, name)
	if version != "" {
		ref += "@" + version
	}
	return ref
}
