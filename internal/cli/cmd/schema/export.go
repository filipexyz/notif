package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
	"github.com/filipexyz/notif/internal/schema/generator"
)

var exportCmd = &cobra.Command{
	Use:   "export <namespace/name>",
	Short: "Export schema to different formats",
	Long: `Export a schema to different programming languages or formats.

Supported formats:
  - json: JSON Schema
  - yaml: YAML Schema
  - typescript: TypeScript interfaces
  - python: Pydantic models
  - go: Go structs
  - rust: Rust structs with serde
  - llm: LLM-friendly markdown

Examples:
  # Export to TypeScript
  notif schema export @filipelabs/agent --format=typescript

  # Export to file
  notif schema export @filipelabs/agent --format=typescript -o ./types/agent.ts

  # Export to Python
  notif schema export @filipelabs/agent --format=python -o ./models/agent.py`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

var (
	exportFormat  string
	exportOutput  string
	exportVersion string
	exportPackage string
)

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, yaml, typescript, python, go, rust, llm)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (stdout if not specified)")
	exportCmd.Flags().StringVarP(&exportVersion, "version", "v", "latest", "Schema version")
	exportCmd.Flags().StringVar(&exportPackage, "package", "main", "Package name (for Go export)")
}

func runExport(cmd *cobra.Command, args []string) error {
	ref, err := schema.ParseSchemaRef(args[0])
	if err != nil {
		return fmt.Errorf("invalid schema reference: %w", err)
	}

	// Override version if specified
	if exportVersion != "latest" {
		ref.Version = exportVersion
	}

	// Load schema from storage
	storage, err := schema.NewStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	s, _, _, err := storage.Load(ref.Namespace, ref.Name, ref.Version)
	if err != nil {
		return fmt.Errorf("schema not installed: %w (try: notif schema install %s)", err, ref.String())
	}

	// Generate output based on format
	var output string
	switch strings.ToLower(exportFormat) {
	case "json":
		output, err = exportJSON(s, ref.Namespace)
	case "yaml":
		output, err = exportYAML(s)
	case "typescript", "ts":
		output, err = generator.GenerateTypeScript(s)
	case "python", "py":
		output, err = generator.GeneratePython(s)
	case "go", "golang":
		output, err = generator.GenerateGo(s, exportPackage)
	case "rust", "rs":
		output, err = generator.GenerateRust(s)
	case "llm", "markdown", "md":
		output, err = generator.GenerateLLM(s, ref.Namespace)
	default:
		return fmt.Errorf("unsupported format: %s", exportFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to generate output: %w", err)
	}

	// Write output
	if exportOutput == "" {
		// Write to stdout
		fmt.Println(output)
	} else {
		// Ensure directory exists
		dir := filepath.Dir(exportOutput)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// Write to file
		if err := os.WriteFile(exportOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Printf("✓ Exported to %s\n", exportOutput)
	}

	return nil
}

func exportJSON(s *schema.Schema, namespace string) (string, error) {
	jsonSchema, err := schema.Convert(s, namespace, "")
	if err != nil {
		return "", err
	}

	// Pretty print
	data, err := json.MarshalIndent(jsonSchema, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func exportYAML(s *schema.Schema) (string, error) {
	// Convert back to YAML (simplified)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	sb.WriteString(fmt.Sprintf("version: %s\n", s.Version))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	}
	sb.WriteString("fields:\n")

	for fieldName, field := range s.Fields {
		sb.WriteString(fmt.Sprintf("  %s:\n", fieldName))
		sb.WriteString(fmt.Sprintf("    type: %s\n", field.Type))
		if field.Required {
			sb.WriteString("    required: true\n")
		}
		if field.Description != "" {
			sb.WriteString(fmt.Sprintf("    description: %s\n", field.Description))
		}
		if field.Default != nil {
			sb.WriteString(fmt.Sprintf("    default: %v\n", field.Default))
		}
	}

	return sb.String(), nil
}
