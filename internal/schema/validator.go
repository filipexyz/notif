package schema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	// validFieldName matches valid field names (alphanumeric, underscore, hyphen).
	validFieldName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)

	// validSchemaName matches valid schema names.
	validSchemaName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	// validTypes are all supported field types.
	validTypes = map[string]bool{
		"string":   true,
		"integer":  true,
		"number":   true,
		"boolean":  true,
		"array":    true,
		"object":   true,
		"enum":     true,
		"datetime": true,
		"date":     true,
		"time":     true,
		"email":    true,
		"uri":      true,
		"url":      true,
		"uuid":     true,
	}
)

// Validate validates a schema and returns a ValidationResult.
func Validate(schema *Schema, strict bool) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if schema == nil {
		result.AddError("", "schema is nil")
		return result
	}

	// Validate name
	if schema.Name == "" {
		result.AddError("name", "name is required")
	} else if !validSchemaName.MatchString(schema.Name) {
		result.AddError("name", "name must start with lowercase letter and contain only lowercase letters, numbers, and hyphens")
	}

	// Validate version
	if schema.Version == "" {
		result.AddError("version", "version is required")
	} else {
		if _, err := semver.NewVersion(schema.Version); err != nil {
			result.AddError("version", fmt.Sprintf("invalid semver format: %v", err))
		}
	}

	// Validate description
	if schema.Description == "" {
		result.AddWarning("description", "description is recommended")
	}

	// Validate fields
	if len(schema.Fields) == 0 {
		result.AddWarning("fields", "schema has no fields")
	}

	seenFields := make(map[string]bool)
	for name, field := range schema.Fields {
		// Check for duplicate fields (case-insensitive)
		lowerName := strings.ToLower(name)
		if seenFields[lowerName] {
			result.AddError(name, "duplicate field name (case-insensitive)")
			continue
		}
		seenFields[lowerName] = true

		// Validate field name
		if !validFieldName.MatchString(name) {
			result.AddError(name, "field name must start with letter or underscore and contain only alphanumeric, underscore, or hyphen")
			continue
		}

		// Validate field
		validateField(name, field, result, make(map[string]bool), 0)
	}

	// In strict mode, warnings count as errors
	if strict && len(result.Warnings) > 0 {
		result.Valid = false
	}

	return result
}

// validateField validates a single field recursively.
func validateField(name string, field *Field, result *ValidationResult, visited map[string]bool, depth int) {
	if field == nil {
		result.AddError(name, "field is nil")
		return
	}

	// Check for circular references
	if visited[name] {
		result.AddError(name, "circular reference detected")
		return
	}
	visited[name] = true
	defer delete(visited, name)

	// Check max depth
	if depth > 20 {
		result.AddError(name, "maximum nesting depth exceeded (20 levels)")
		return
	}

	// Validate type
	if field.Type == "" {
		result.AddError(name, "type is required")
		return
	}

	if !validTypes[field.Type] {
		// Try to suggest similar type
		suggestion := suggestType(field.Type)
		if suggestion != "" {
			result.AddError(name, fmt.Sprintf("invalid type %q (did you mean %q?)", field.Type, suggestion))
		} else {
			result.AddError(name, fmt.Sprintf("invalid type %q", field.Type))
		}
		return
	}

	// Type-specific validation
	switch field.Type {
	case "array":
		if field.Items == nil {
			result.AddWarning(name, "array type should specify items")
		} else {
			// Validate items
			switch items := field.Items.(type) {
			case string:
				if !validTypes[items] {
					result.AddError(name+".items", fmt.Sprintf("invalid items type %q", items))
				}
			case *Field:
				validateField(name+"[]", items, result, visited, depth+1)
			case map[string]any:
				// Try to convert and validate
				if nestedField, err := mapToField(items); err == nil {
					validateField(name+"[]", nestedField, result, visited, depth+1)
				}
			}
		}

		if field.MinItems > 0 && field.MinItems < 0 {
			result.AddError(name, "minItems cannot be negative")
		}
		if field.MaxItems > 0 && field.MaxItems < 0 {
			result.AddError(name, "maxItems cannot be negative")
		}
		if field.MinItems > 0 && field.MaxItems > 0 && field.MinItems > field.MaxItems {
			result.AddError(name, "minItems cannot be greater than maxItems")
		}

	case "object":
		if field.Properties != nil {
			for propName, propField := range field.Properties {
				validateField(name+"."+propName, propField, result, visited, depth+1)
			}
		}

	case "enum":
		if len(field.Values) == 0 {
			result.AddError(name, "enum must have at least one value")
		}

		// Check for duplicates
		seen := make(map[string]bool)
		for _, v := range field.Values {
			if seen[v] {
				result.AddError(name, fmt.Sprintf("duplicate enum value %q", v))
			}
			seen[v] = true
		}

	case "string":
		if field.MinLength > 0 && field.MinLength < 0 {
			result.AddError(name, "minLength cannot be negative")
		}
		if field.MaxLength > 0 && field.MaxLength < 0 {
			result.AddError(name, "maxLength cannot be negative")
		}
		if field.MinLength > 0 && field.MaxLength > 0 && field.MinLength > field.MaxLength {
			result.AddError(name, "minLength cannot be greater than maxLength")
		}
		if field.Pattern != "" {
			if _, err := regexp.Compile(field.Pattern); err != nil {
				result.AddError(name, fmt.Sprintf("invalid regex pattern: %v", err))
			}
		}

	case "integer", "number":
		if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
			result.AddError(name, "min cannot be greater than max")
		}
	}

	// Validate default value type
	if field.Default != nil {
		if !isValidDefault(field.Default, field.Type) {
			result.AddError(name, fmt.Sprintf("default value type doesn't match field type %q", field.Type))
		}
	}

	// Warn about description
	if field.Description == "" {
		result.AddWarning(name, "description is recommended")
	}
}

// isValidDefault checks if a default value matches the field type.
func isValidDefault(value any, fieldType string) bool {
	switch fieldType {
	case "string", "datetime", "date", "time", "email", "uri", "url", "uuid":
		_, ok := value.(string)
		return ok
	case "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float64:
			// YAML/JSON may parse as float64
			f := value.(float64)
			return f == float64(int64(f))
		}
		return false
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		// Check if it's a slice
		switch value.(type) {
		case []any, []string, []int, []float64, []bool:
			return true
		}
		return false
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "enum":
		_, ok := value.(string)
		return ok
	}
	return false
}

// suggestType suggests a similar type if there's a typo.
func suggestType(input string) string {
	input = strings.ToLower(input)

	// Common typos
	suggestions := map[string]string{
		"str":     "string",
		"text":    "string",
		"int":     "integer",
		"float":   "number",
		"double":  "number",
		"bool":    "boolean",
		"arr":     "array",
		"list":    "array",
		"obj":     "object",
		"dict":    "object",
		"map":     "object",
		"timestamp": "datetime",
	}

	if suggestion, ok := suggestions[input]; ok {
		return suggestion
	}

	// Check for close matches using Levenshtein distance
	minDist := 3
	var bestMatch string
	for validType := range validTypes {
		dist := levenshtein(input, validType)
		if dist < minDist {
			minDist = dist
			bestMatch = validType
		}
	}

	return bestMatch
}

// levenshtein calculates the Levenshtein distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
