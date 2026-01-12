package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PublishOptions contains options for publishing a schema
type PublishOptions struct {
	SchemaFile string
	Namespace  string
	Name       string
	OutputDir  string
	DryRun     bool
}

// PublishResult contains the result of a publish operation
type PublishResult struct {
	Namespace string
	Name      string
	Version   string
	OutputDir string
	Files     []string
}

// Publish prepares a schema for publishing to the registry
func Publish(opts PublishOptions) (*PublishResult, error) {
	// Parse schema
	schema, err := Parse(opts.SchemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Validate schema
	result := Validate(schema, true) // strict mode
	if !result.Valid {
		return nil, fmt.Errorf("schema validation failed: %d errors", len(result.Errors))
	}

	// Use schema name if not provided
	if opts.Name == "" {
		opts.Name = schema.Name
	}

	// Check namespace format
	if !isValidNamespace(opts.Namespace) {
		return nil, fmt.Errorf("invalid namespace: must be alphanumeric with optional hyphens")
	}

	// Check name format
	if !isValidName(opts.Name) {
		return nil, fmt.Errorf("invalid name: must be alphanumeric with optional hyphens")
	}

	// Check version format
	if schema.Version == "" {
		return nil, fmt.Errorf("schema must have a version")
	}

	// Check with registry
	registry := NewRegistry()

	// Check namespace availability
	available, err := registry.CheckNamespaceAvailable(opts.Namespace)
	if err != nil {
		// Network error - warn but continue
		fmt.Printf("⚠ Warning: Could not verify namespace availability: %v\n", err)
	} else if !available {
		// Namespace exists - this is fine, user might own it
		fmt.Printf("ℹ Namespace @%s exists (CI will verify ownership)\n", opts.Namespace)
	}

	// Check version doesn't exist
	exists, err := registry.CheckVersionExists(opts.Namespace, opts.Name, schema.Version)
	if err != nil {
		// Network error - warn but continue
		fmt.Printf("⚠ Warning: Could not verify version availability: %v\n", err)
	} else if exists {
		return nil, fmt.Errorf("version @%s/%s@%s already exists", opts.Namespace, opts.Name, schema.Version)
	}

	if opts.DryRun {
		return &PublishResult{
			Namespace: opts.Namespace,
			Name:      opts.Name,
			Version:   schema.Version,
		}, nil
	}

	// Convert to JSON Schema
	jsonSchema, err := Convert(schema, opts.Namespace, "")
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(opts.OutputDir, opts.Namespace, opts.Name, schema.Version)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	files := []string{}

	// Write schema.json
	schemaPath := filepath.Join(outputDir, "schema.json")
	schemaJSON, err := json.MarshalIndent(jsonSchema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write schema.json: %w", err)
	}
	files = append(files, schemaPath)

	// Generate README.md
	readmePath := filepath.Join(outputDir, "README.md")
	readme := generateREADME(schema, opts.Namespace, opts.Name)
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return nil, fmt.Errorf("failed to write README.md: %w", err)
	}
	files = append(files, readmePath)

	return &PublishResult{
		Namespace: opts.Namespace,
		Name:      opts.Name,
		Version:   schema.Version,
		OutputDir: outputDir,
		Files:     files,
	}, nil
}

// generateREADME generates a README.md from a schema
func generateREADME(schema *Schema, namespace, name string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# @%s/%s\n\n", namespace, name))

	if schema.Description != "" {
		sb.WriteString(schema.Description)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Version\n\n")
	sb.WriteString(fmt.Sprintf("`%s`\n\n", schema.Version))

	if len(schema.Fields) > 0 {
		sb.WriteString("## Fields\n\n")
		sb.WriteString("| Field | Type | Required | Description |\n")
		sb.WriteString("|-------|------|----------|-------------|\n")

		for fieldName, field := range schema.Fields {
			req := ""
			if field.Required {
				req = "✓"
			}
			desc := field.Description
			if desc == "" {
				desc = "-"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n",
				fieldName, field.Type, req, desc))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Installation\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("notif schema install @%s/%s\n", namespace, name))
	sb.WriteString("```\n\n")

	sb.WriteString("## Usage\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("# Emit an event with this schema\n"))
	sb.WriteString(fmt.Sprintf("notif emit your.topic '{\"your\":\"data\"}'\n"))
	sb.WriteString("```\n\n")

	sb.WriteString("## Example\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(generateExample(schema))
	sb.WriteString("\n```\n")

	return sb.String()
}

// generateExample generates a JSON example from a schema
func generateExample(schema *Schema) string {
	example := make(map[string]interface{})

	for fieldName, field := range schema.Fields {
		if field.Default != nil {
			example[fieldName] = field.Default
		} else {
			example[fieldName] = getExampleValue(*field)
		}
	}

	data, _ := json.MarshalIndent(example, "", "  ")
	return string(data)
}

// getExampleValue returns an example value for a field type
func getExampleValue(field Field) interface{} {
	switch field.Type {
	case "string":
		if field.Format == "email" {
			return "user@example.com"
		}
		if field.Format == "uri" {
			return "https://example.com"
		}
		if field.Format == "uuid" {
			return "123e4567-e89b-12d3-a456-426614174000"
		}
		if field.Format == "date" {
			return "2024-01-15"
		}
		if field.Format == "time" {
			return "14:30:00"
		}
		if field.Format == "date-time" {
			return "2024-01-15T14:30:00Z"
		}
		return "example"
	case "integer":
		return 42
	case "number":
		return 3.14
	case "boolean":
		return true
	case "array":
		return []interface{}{"item1", "item2"}
	case "object":
		return map[string]interface{}{}
	default:
		return nil
	}
}

// GeneratePublishInstructions generates instructions for publishing
func GeneratePublishInstructions(result *PublishResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\nSchema is ready to publish!\n\n"))
	sb.WriteString(fmt.Sprintf("To publish @%s/%s@%s:\n\n", result.Namespace, result.Name, result.Version))
	sb.WriteString("1. Fork https://github.com/notifsh/schemas (if not already)\n")
	sb.WriteString("2. Clone your fork\n")
	sb.WriteString(fmt.Sprintf("3. Create branch: git checkout -b schema/%s/%s/%s\n", result.Namespace, result.Name, result.Version))
	sb.WriteString(fmt.Sprintf("4. Create directory: mkdir -p schemas/%s/%s/%s\n", result.Namespace, result.Name, result.Version))
	sb.WriteString("5. Copy the generated files (see below)\n")
	sb.WriteString(fmt.Sprintf("6. Commit: git commit -m \"feat: add @%s/%s@%s\"\n", result.Namespace, result.Name, result.Version))
	sb.WriteString(fmt.Sprintf("7. Push: git push origin schema/%s/%s/%s\n", result.Namespace, result.Name, result.Version))
	sb.WriteString("8. Open PR at https://github.com/notifsh/schemas/pulls\n\n")
	sb.WriteString(fmt.Sprintf("Generated files saved to: %s\n", result.OutputDir))
	for _, file := range result.Files {
		sb.WriteString(fmt.Sprintf("  - %s\n", filepath.Base(file)))
	}
	sb.WriteString(fmt.Sprintf("\nCopy these files to your fork at: schemas/%s/%s/%s/\n", result.Namespace, result.Name, result.Version))

	return sb.String()
}

// Helper functions
func isValidNamespace(s string) bool {
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	for i, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (i > 0 && r == '-')) {
			return false
		}
	}
	return true
}

func isValidName(s string) bool {
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	for i, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (i > 0 && (r == '-' || r == '_'))) {
			return false
		}
	}
	return true
}
