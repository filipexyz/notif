package codegen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONSchema represents a JSON Schema document.
type JSONSchema struct {
	Type        interface{}            `json:"type,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Description string                 `json:"description,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Definitions map[string]*JSONSchema `json:"definitions,omitempty"`
	Defs        map[string]*JSONSchema `json:"$defs,omitempty"` // JSON Schema draft-2020-12

	// Validation keywords
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
	MinItems  *int     `json:"minItems,omitempty"`
	MaxItems  *int     `json:"maxItems,omitempty"`

	// Composition
	OneOf   []*JSONSchema `json:"oneOf,omitempty"`
	AnyOf   []*JSONSchema `json:"anyOf,omitempty"`
	AllOf   []*JSONSchema `json:"allOf,omitempty"`
	Const   interface{}   `json:"const,omitempty"`
	Default interface{}   `json:"default,omitempty"`

	// Additional
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`
	Nullable             bool        `json:"nullable,omitempty"`
}

// SchemaParser converts JSON Schema to IR.
type SchemaParser struct {
	rootSchema  *JSONSchema
	definitions map[string]*Type
}

// ParseJSONSchema parses a JSON Schema into the intermediate representation.
func ParseJSONSchema(data []byte, name, topic, version string) (*Schema, error) {
	var js JSONSchema
	if err := json.Unmarshal(data, &js); err != nil {
		return nil, fmt.Errorf("failed to parse JSON schema: %w", err)
	}

	parser := &SchemaParser{
		rootSchema:  &js,
		definitions: make(map[string]*Type),
	}

	// Parse definitions first
	defs := js.Definitions
	if defs == nil {
		defs = js.Defs
	}
	if defs != nil {
		for defName, defSchema := range defs {
			t, err := parser.parseType(defSchema, toPascalCase(defName))
			if err != nil {
				return nil, fmt.Errorf("failed to parse definition %s: %w", defName, err)
			}
			parser.definitions[defName] = t
		}
	}

	// Parse root type
	root, err := parser.parseType(&js, toPascalCase(name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse root type: %w", err)
	}

	return &Schema{
		Name:        name,
		Topic:       topic,
		Version:     version,
		Description: js.Description,
		Root:        root,
		Definitions: parser.definitions,
	}, nil
}

func (p *SchemaParser) parseType(js *JSONSchema, suggestedName string) (*Type, error) {
	if js == nil {
		return NewAnyType(), nil
	}

	// Handle $ref
	if js.Ref != "" {
		refName := extractRefName(js.Ref)
		return NewRefType(refName), nil
	}

	// Handle oneOf/anyOf (treat as any for now, could be improved to union types)
	if len(js.OneOf) > 0 || len(js.AnyOf) > 0 {
		// Check if it's a nullable type pattern: { oneOf: [{type: "something"}, {type: "null"}] }
		schemas := js.OneOf
		if len(schemas) == 0 {
			schemas = js.AnyOf
		}
		if len(schemas) == 2 {
			var nonNullSchema *JSONSchema
			hasNull := false
			for _, s := range schemas {
				if getTypeString(s.Type) == "null" {
					hasNull = true
				} else {
					nonNullSchema = s
				}
			}
			if hasNull && nonNullSchema != nil {
				t, err := p.parseType(nonNullSchema, suggestedName)
				if err != nil {
					return nil, err
				}
				t.Nullable = true
				return t, nil
			}
		}
		// General case: use any
		return NewAnyType(), nil
	}

	// Handle allOf (merge properties)
	if len(js.AllOf) > 0 {
		merged := &Type{
			Kind:        KindObject,
			Name:        suggestedName,
			Description: js.Description,
		}
		propMap := make(map[string]Property)
		for _, s := range js.AllOf {
			t, err := p.parseType(s, suggestedName)
			if err != nil {
				return nil, err
			}
			if t.Kind == KindObject {
				for _, prop := range t.Properties {
					propMap[prop.JSONName] = prop
				}
			}
		}
		for _, prop := range propMap {
			merged.Properties = append(merged.Properties, prop)
		}
		return merged, nil
	}

	// Handle enum (with type: string)
	if len(js.Enum) > 0 {
		var enumValues []string
		for _, v := range js.Enum {
			if s, ok := v.(string); ok {
				enumValues = append(enumValues, s)
			}
		}
		if len(enumValues) > 0 {
			t := NewEnumType(enumValues)
			t.Description = js.Description
			return t, nil
		}
	}

	// Handle const (treat as enum with single value)
	if js.Const != nil {
		if s, ok := js.Const.(string); ok {
			t := NewEnumType([]string{s})
			t.Description = js.Description
			return t, nil
		}
	}

	typeStr := getTypeString(js.Type)

	switch typeStr {
	case "object":
		return p.parseObjectType(js, suggestedName)
	case "array":
		return p.parseArrayType(js, suggestedName)
	case "string":
		t := NewStringType()
		t.Description = js.Description
		t.Format = js.Format
		t.MinLength = js.MinLength
		t.MaxLength = js.MaxLength
		t.Pattern = js.Pattern
		t.Nullable = js.Nullable
		return t, nil
	case "number":
		t := NewNumberType()
		t.Description = js.Description
		t.Minimum = js.Minimum
		t.Maximum = js.Maximum
		t.Nullable = js.Nullable
		return t, nil
	case "integer":
		t := NewIntegerType()
		t.Description = js.Description
		t.Minimum = js.Minimum
		t.Maximum = js.Maximum
		t.Nullable = js.Nullable
		return t, nil
	case "boolean":
		t := NewBooleanType()
		t.Description = js.Description
		t.Nullable = js.Nullable
		return t, nil
	case "null":
		t := NewAnyType()
		t.Nullable = true
		return t, nil
	case "":
		// No type specified, check if it has properties (implicit object)
		if js.Properties != nil {
			return p.parseObjectType(js, suggestedName)
		}
		// Otherwise, it's any
		return NewAnyType(), nil
	default:
		return NewAnyType(), nil
	}
}

func (p *SchemaParser) parseObjectType(js *JSONSchema, name string) (*Type, error) {
	t := NewObjectType(name)
	t.Description = js.Description
	t.Nullable = js.Nullable

	requiredSet := make(map[string]bool)
	for _, r := range js.Required {
		requiredSet[r] = true
	}

	for propName, propSchema := range js.Properties {
		propTypeName := name + toPascalCase(propName)
		propType, err := p.parseType(propSchema, propTypeName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse property %s: %w", propName, err)
		}

		// If it's a nested object, give it a name and add to definitions
		if propType.Kind == KindObject && propType.Name == propTypeName && len(propType.Properties) > 0 {
			p.definitions[propTypeName] = propType
		}

		prop := Property{
			Name:        toPascalCase(propName),
			JSONName:    propName,
			Type:        propType,
			Required:    requiredSet[propName],
			Description: propSchema.Description,
		}
		t.Properties = append(t.Properties, prop)
	}

	return t, nil
}

func (p *SchemaParser) parseArrayType(js *JSONSchema, suggestedName string) (*Type, error) {
	// Derive item type name from array type name
	// OrderPlacedItems -> OrderPlacedItem
	// Items -> Item
	itemsName := strings.TrimSuffix(suggestedName, "s")
	if itemsName == suggestedName {
		// No 's' suffix, add 'Item'
		itemsName = suggestedName + "Item"
	}

	items, err := p.parseType(js.Items, itemsName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse array items: %w", err)
	}

	// If items is an object, add it to definitions
	if items.Kind == KindObject && items.Name != "" && len(items.Properties) > 0 {
		p.definitions[items.Name] = items
	}

	t := NewArrayType(items)
	t.Description = js.Description
	t.MinItems = js.MinItems
	t.MaxItems = js.MaxItems
	t.Nullable = js.Nullable

	return t, nil
}

func getTypeString(t interface{}) string {
	switch v := t.(type) {
	case string:
		return v
	case []interface{}:
		// Handle ["string", "null"] format
		for _, item := range v {
			if s, ok := item.(string); ok && s != "null" {
				return s
			}
		}
		return ""
	default:
		return ""
	}
}

func extractRefName(ref string) string {
	// Handle #/definitions/Name or #/$defs/Name
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}

// toPascalCase converts a string to PascalCase.
func toPascalCase(s string) string {
	// Handle kebab-case and snake_case
	s = strings.ReplaceAll(s, "-", "_")
	parts := strings.Split(s, "_")

	var result strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			result.WriteString(part[1:])
		}
	}
	return result.String()
}

// toCamelCase converts a string to camelCase.
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if pascal == "" {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// toSnakeCase converts a string to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
