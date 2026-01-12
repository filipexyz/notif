package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/spf13/cobra"
)

var (
	initName   string
	initOutput string
	initFormat string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new schema file from template",
		Long: `Create a new schema file from template.

The template includes basic fields and comments to help you get started.

Examples:
  # Create schema.yaml in current directory
  notif schema init

  # Create agent.yaml in current directory
  notif schema init --name=agent

  # Create schema in specific directory
  notif schema init --output=./schemas/

  # Create JSON schema instead of YAML
  notif schema init --format=json`,
		Run: runInit,
	}

	cmd.Flags().StringVar(&initName, "name", "schema", "schema name")
	cmd.Flags().StringVarP(&initOutput, "output", "o", ".", "output directory")
	cmd.Flags().StringVar(&initFormat, "format", "yaml", "output format (yaml or json)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) {
	out := output.New(false)

	// Validate format
	if initFormat != "yaml" && initFormat != "json" {
		out.Error("Invalid format: %s (use 'yaml' or 'json')", initFormat)
		os.Exit(1)
	}

	// Generate template
	tmpl := schema.GenerateTemplate(initName)

	// Determine output path
	var filename string
	if initFormat == "json" {
		filename = initName + ".json"
	} else {
		filename = initName + ".yaml"
	}

	outputPath := filepath.Join(initOutput, filename)

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil {
		out.Error("File already exists: %s", outputPath)
		os.Exit(1)
	}

	// Create output directory if needed
	if err := os.MkdirAll(initOutput, 0755); err != nil {
		out.Error("Failed to create output directory: %v", err)
		os.Exit(1)
	}

	// Marshal template
	var data []byte
	var err error
	if initFormat == "json" {
		data, err = schema.MarshalJSON(tmpl)
	} else {
		data, err = schema.MarshalYAML(tmpl)
	}
	if err != nil {
		out.Error("Failed to generate template: %v", err)
		os.Exit(1)
	}

	// Add helpful comments for YAML
	if initFormat == "yaml" {
		header := `# Schema definition for notif.sh
# Docs: https://notif.sh/docs/schemas

`
		data = append([]byte(header), data...)
	}

	// Write file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		out.Error("Failed to write file: %v", err)
		os.Exit(1)
	}

	out.Success("Created %s", outputPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the schema file to define your event structure")
	fmt.Println("  2. Validate it: notif schema validate " + filename)
	fmt.Println("  3. Install it locally: notif schema install " + filename)
}
