package schema

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var publishCmd = &cobra.Command{
	Use:   "publish <namespace/name> <file>",
	Short: "Publish schema via PR to registry",
	Long: `Validates a schema and generates files for publishing to the notif-schemas repository.

The command will:
1. Validate the schema locally
2. Check namespace and version availability
3. Convert YAML to JSON Schema
4. Generate README.md from schema
5. Save files to output directory
6. Print step-by-step instructions for opening a PR

Examples:
  # Publish a schema
  notif schema publish @filipelabs/agent ./agent.yaml

  # Dry run (validate only)
  notif schema publish @filipelabs/agent ./agent.yaml --dry-run

  # Custom output directory
  notif schema publish @filipelabs/agent ./agent.yaml -o ./publish/`,
	Args: cobra.ExactArgs(2),
	RunE: runPublish,
}

var (
	publishOutput string
	publishDryRun bool
)

func init() {
	publishCmd.Flags().StringVarP(&publishOutput, "output", "o", "./.notif-publish", "Output directory for generated files")
	publishCmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "Validate only, don't generate files")
}

func runPublish(cmd *cobra.Command, args []string) error {
	ref, err := schema.ParseSchemaRef(args[0])
	if err != nil {
		return fmt.Errorf("invalid schema reference: %w", err)
	}

	schemaFile := args[1]

	// Check if file exists
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", schemaFile)
	}

	fmt.Println("Validating schema...")

	opts := schema.PublishOptions{
		SchemaFile: schemaFile,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
		OutputDir:  publishOutput,
		DryRun:     publishDryRun,
	}

	result, err := schema.Publish(opts)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	fmt.Println("✓ Validation complete")

	if publishDryRun {
		fmt.Printf("\n✓ Dry run complete. Schema @%s/%s@%s is valid and ready to publish.\n",
			result.Namespace, result.Name, result.Version)
		return nil
	}

	fmt.Println(schema.GeneratePublishInstructions(result))

	return nil
}
