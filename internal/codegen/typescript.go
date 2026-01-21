package codegen

import (
	"fmt"
	"sort"
	"strings"
)

// TypeScriptGenerator generates TypeScript code with Zod validators.
type TypeScriptGenerator struct {
	options TypeScriptOptions
}

// NewTypeScriptGenerator creates a new TypeScript generator.
func NewTypeScriptGenerator(options TypeScriptOptions) *TypeScriptGenerator {
	return &TypeScriptGenerator{options: options}
}

// Generate generates TypeScript code for a schema.
func (g *TypeScriptGenerator) Generate(schema *Schema) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString("import { z } from 'zod';\n\n")

	// Schema metadata comment
	b.WriteString(fmt.Sprintf("// Schema: %s\n", schema.Name))
	if schema.Topic != "" {
		b.WriteString(fmt.Sprintf("// Topic: %s\n", schema.Topic))
	}
	if schema.Version != "" {
		b.WriteString(fmt.Sprintf("// Version: %s\n", schema.Version))
	}
	if schema.Description != "" {
		b.WriteString(fmt.Sprintf("// %s\n", schema.Description))
	}
	b.WriteString("\n")

	// Generate definitions first (nested types)
	if len(schema.Definitions) > 0 {
		// Sort for deterministic output
		var defNames []string
		for name := range schema.Definitions {
			defNames = append(defNames, name)
		}
		sort.Strings(defNames)

		for _, name := range defNames {
			def := schema.Definitions[name]
			// Skip if it's the same as root
			if name == toPascalCase(schema.Name) {
				continue
			}
			zodCode := g.generateZodType(def)
			b.WriteString(fmt.Sprintf("export const %sSchema = %s;\n\n", name, zodCode))
			b.WriteString(fmt.Sprintf("export type %s = z.infer<typeof %sSchema>;\n\n", name, name))
		}
	}

	// Generate root schema
	rootName := toPascalCase(schema.Name)
	rootZod := g.generateZodType(schema.Root)
	b.WriteString(fmt.Sprintf("export const %sSchema = %s;\n\n", rootName, rootZod))

	// Type inference
	b.WriteString(fmt.Sprintf("export type %s = z.infer<typeof %sSchema>;\n\n", rootName, rootName))

	// Validation helper
	b.WriteString(fmt.Sprintf("// Validation helper\n"))
	b.WriteString(fmt.Sprintf("export function validate%s(data: unknown): %s {\n", rootName, rootName))
	b.WriteString(fmt.Sprintf("  return %sSchema.parse(data);\n", rootName))
	b.WriteString("}\n\n")

	// Type guard
	b.WriteString(fmt.Sprintf("// Type guard\n"))
	b.WriteString(fmt.Sprintf("export function is%s(data: unknown): data is %s {\n", rootName, rootName))
	b.WriteString(fmt.Sprintf("  return %sSchema.safeParse(data).success;\n", rootName))
	b.WriteString("}\n")

	return b.String(), nil
}

func (g *TypeScriptGenerator) generateZodType(t *Type) string {
	if t == nil {
		return "z.unknown()"
	}

	var base string

	switch t.Kind {
	case KindObject:
		base = g.generateZodObject(t)
	case KindArray:
		itemsZod := g.generateZodType(t.Items)
		base = fmt.Sprintf("z.array(%s)", itemsZod)
		if t.MinItems != nil {
			base = fmt.Sprintf("%s.min(%d)", base, *t.MinItems)
		}
		if t.MaxItems != nil {
			base = fmt.Sprintf("%s.max(%d)", base, *t.MaxItems)
		}
	case KindString:
		base = g.generateZodString(t)
	case KindNumber:
		base = "z.number()"
		if t.Minimum != nil {
			base = fmt.Sprintf("%s.min(%v)", base, *t.Minimum)
		}
		if t.Maximum != nil {
			base = fmt.Sprintf("%s.max(%v)", base, *t.Maximum)
		}
	case KindInteger:
		base = "z.number().int()"
		if t.Minimum != nil {
			base = fmt.Sprintf("%s.min(%v)", base, *t.Minimum)
		}
		if t.Maximum != nil {
			base = fmt.Sprintf("%s.max(%v)", base, *t.Maximum)
		}
	case KindBoolean:
		base = "z.boolean()"
	case KindEnum:
		base = g.generateZodEnum(t)
	case KindRef:
		base = fmt.Sprintf("%sSchema", toPascalCase(t.Ref))
	case KindAny:
		base = "z.unknown()"
	default:
		base = "z.unknown()"
	}

	if t.Nullable {
		base = fmt.Sprintf("%s.nullable()", base)
	}

	return base
}

func (g *TypeScriptGenerator) generateZodObject(t *Type) string {
	if len(t.Properties) == 0 {
		return "z.object({})"
	}

	var b strings.Builder
	b.WriteString("z.object({\n")

	// Sort properties for deterministic output
	props := make([]Property, len(t.Properties))
	copy(props, t.Properties)
	sort.Slice(props, func(i, j int) bool {
		return props[i].JSONName < props[j].JSONName
	})

	for i, prop := range props {
		propZod := g.generateZodType(prop.Type)

		// Add description if present
		if prop.Description != "" {
			propZod = fmt.Sprintf("%s.describe(%q)", propZod, prop.Description)
		}

		// Make optional if not required
		if !prop.Required {
			propZod = fmt.Sprintf("%s.optional()", propZod)
		}

		b.WriteString(fmt.Sprintf("  %s: %s", prop.JSONName, propZod))
		if i < len(props)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}

	b.WriteString("})")
	return b.String()
}

func (g *TypeScriptGenerator) generateZodString(t *Type) string {
	base := "z.string()"

	// Handle specific formats
	switch t.Format {
	case "email":
		base = "z.string().email()"
	case "uri", "url":
		base = "z.string().url()"
	case "uuid":
		base = "z.string().uuid()"
	case "date-time":
		base = "z.string().datetime()"
	case "date":
		base = "z.string().date()"
	case "time":
		base = "z.string().time()"
	case "ip", "ipv4":
		base = "z.string().ip({ version: 'v4' })"
	case "ipv6":
		base = "z.string().ip({ version: 'v6' })"
	}

	// Add constraints
	if t.MinLength != nil {
		base = fmt.Sprintf("%s.min(%d)", base, *t.MinLength)
	}
	if t.MaxLength != nil {
		base = fmt.Sprintf("%s.max(%d)", base, *t.MaxLength)
	}
	if t.Pattern != "" {
		base = fmt.Sprintf("%s.regex(/%s/)", base, escapeRegex(t.Pattern))
	}

	return base
}

func (g *TypeScriptGenerator) generateZodEnum(t *Type) string {
	if len(t.Enum) == 0 {
		return "z.string()"
	}

	if len(t.Enum) == 1 {
		return fmt.Sprintf("z.literal(%q)", t.Enum[0])
	}

	var quoted []string
	for _, v := range t.Enum {
		quoted = append(quoted, fmt.Sprintf("%q", v))
	}
	return fmt.Sprintf("z.enum([%s])", strings.Join(quoted, ", "))
}

// GenerateBarrelFile generates an index.ts barrel file that exports all schemas.
func (g *TypeScriptGenerator) GenerateBarrelFile(schemas []string) string {
	var b strings.Builder

	b.WriteString("// Auto-generated barrel file for notif.sh schemas\n")
	b.WriteString("// Do not edit manually\n\n")

	sort.Strings(schemas)

	for _, name := range schemas {
		filename := toSnakeCase(name)
		if g.options.Exports == "default" {
			b.WriteString(fmt.Sprintf("export { default as %s } from './%s';\n", toPascalCase(name), filename))
		} else {
			b.WriteString(fmt.Sprintf("export * from './%s';\n", filename))
		}
	}

	return b.String()
}

func escapeRegex(pattern string) string {
	// Escape forward slashes for JavaScript regex literals
	return strings.ReplaceAll(pattern, "/", "\\/")
}
