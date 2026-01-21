package codegen

import (
	"fmt"
	"sort"
	"strings"
)

// GoGenerator generates Go struct code.
type GoGenerator struct {
	options GoOptions
}

// NewGoGenerator creates a new Go generator.
func NewGoGenerator(options GoOptions) *GoGenerator {
	return &GoGenerator{options: options}
}

// Generate generates Go code for a schema.
func (g *GoGenerator) Generate(schema *Schema) (string, error) {
	var b strings.Builder

	// Package declaration
	b.WriteString(fmt.Sprintf("package %s\n\n", g.options.Package))

	// Schema metadata comment
	b.WriteString(fmt.Sprintf("// %s represents data for %s events\n", toPascalCase(schema.Name), schema.Topic))
	if schema.Version != "" {
		b.WriteString(fmt.Sprintf("// Schema: %s, Version: %s\n", schema.Name, schema.Version))
	}
	if schema.Description != "" {
		b.WriteString(fmt.Sprintf("// %s\n", schema.Description))
	}

	// Generate definitions first (nested types) - in reverse order so dependencies come first
	if len(schema.Definitions) > 0 {
		var defNames []string
		for name := range schema.Definitions {
			defNames = append(defNames, name)
		}
		sort.Strings(defNames)

		// Generate nested types after the main type
		var nestedTypes []string
		for _, name := range defNames {
			def := schema.Definitions[name]
			// Skip if it's the same as root
			if name == toPascalCase(schema.Name) {
				continue
			}
			code := g.generateStruct(def, name)
			nestedTypes = append(nestedTypes, code)
		}

		// Generate root struct
		rootCode := g.generateStruct(schema.Root, toPascalCase(schema.Name))
		b.WriteString(rootCode)
		b.WriteString("\n")

		// Then nested types
		for _, code := range nestedTypes {
			b.WriteString(code)
			b.WriteString("\n")
		}
	} else {
		// Just generate root struct
		rootCode := g.generateStruct(schema.Root, toPascalCase(schema.Name))
		b.WriteString(rootCode)
	}

	return b.String(), nil
}

func (g *GoGenerator) generateStruct(t *Type, name string) string {
	if t == nil || t.Kind != KindObject {
		return ""
	}

	var b strings.Builder

	// Description comment
	if t.Description != "" {
		b.WriteString(fmt.Sprintf("// %s %s\n", name, t.Description))
	}

	b.WriteString(fmt.Sprintf("type %s struct {\n", name))

	// Sort properties for deterministic output
	props := make([]Property, len(t.Properties))
	copy(props, t.Properties)
	sort.Slice(props, func(i, j int) bool {
		return props[i].JSONName < props[j].JSONName
	})

	for _, prop := range props {
		goType := g.goType(prop.Type)
		jsonTag := g.jsonTag(prop)

		// Add description as comment
		if prop.Description != "" {
			b.WriteString(fmt.Sprintf("\t// %s\n", prop.Description))
		}

		b.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", prop.Name, goType, jsonTag))
	}

	b.WriteString("}\n")
	return b.String()
}

func (g *GoGenerator) goType(t *Type) string {
	if t == nil {
		return "interface{}"
	}

	var base string

	switch t.Kind {
	case KindObject:
		if t.Name != "" {
			base = t.Name
		} else {
			base = "interface{}"
		}
	case KindArray:
		itemType := g.goType(t.Items)
		base = fmt.Sprintf("[]%s", itemType)
	case KindString:
		base = g.goStringType(t)
	case KindNumber:
		base = "float64"
	case KindInteger:
		base = "int"
	case KindBoolean:
		base = "bool"
	case KindEnum:
		base = "string"
	case KindRef:
		base = toPascalCase(t.Ref)
	case KindAny:
		base = "interface{}"
	default:
		base = "interface{}"
	}

	if t.Nullable && !strings.HasPrefix(base, "*") && !strings.HasPrefix(base, "[]") && base != "interface{}" {
		base = "*" + base
	}

	return base
}

func (g *GoGenerator) goStringType(t *Type) string {
	// Could use specific types for certain formats
	switch t.Format {
	case "date-time":
		return "time.Time"
	default:
		return "string"
	}
}

func (g *GoGenerator) jsonTag(prop Property) string {
	tag := prop.JSONName

	switch g.options.JSONTags {
	case "omitempty":
		if !prop.Required {
			tag += ",omitempty"
		}
	case "required":
		// No omitempty
	case "none":
		// No additional tags
	default:
		// Default to omitempty for optional
		if !prop.Required {
			tag += ",omitempty"
		}
	}

	return tag
}

// NeedsTimeImport checks if the generated code needs the time import.
func (g *GoGenerator) NeedsTimeImport(schema *Schema) bool {
	return g.typeNeedsTime(schema.Root) || g.definitionsNeedTime(schema.Definitions)
}

func (g *GoGenerator) typeNeedsTime(t *Type) bool {
	if t == nil {
		return false
	}

	if t.Kind == KindString && t.Format == "date-time" {
		return true
	}

	if t.Kind == KindArray && t.Items != nil {
		return g.typeNeedsTime(t.Items)
	}

	if t.Kind == KindObject {
		for _, prop := range t.Properties {
			if g.typeNeedsTime(prop.Type) {
				return true
			}
		}
	}

	return false
}

func (g *GoGenerator) definitionsNeedTime(defs map[string]*Type) bool {
	for _, t := range defs {
		if g.typeNeedsTime(t) {
			return true
		}
	}
	return false
}

// GenerateWithImports generates Go code with appropriate imports.
func (g *GoGenerator) GenerateWithImports(schema *Schema) (string, error) {
	code, err := g.Generate(schema)
	if err != nil {
		return "", err
	}

	if g.NeedsTimeImport(schema) {
		// Insert import after package declaration
		lines := strings.SplitN(code, "\n", 2)
		if len(lines) == 2 {
			return lines[0] + "\n\nimport \"time\"\n" + lines[1], nil
		}
	}

	return code, nil
}
