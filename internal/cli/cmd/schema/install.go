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
	installForce   bool
	installVersion string
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <file|namespace/name>",
		Short: "Install a schema locally",
		Long: `Install a schema locally for validation and code generation.

The schema will be stored in ~/.notif/schemas/ organized by namespace and name.

Examples:
  # Install from a local file
  notif schema install ./agent.yaml

  # Install from registry
  notif schema install @filipelabs/agent

  # Install specific version
  notif schema install @filipelabs/agent@1.0.0
  # or
  notif schema install @filipelabs/agent --version=1.0.0

  # Overwrite if exists
  notif schema install ./agent.yaml --force`,
		Args: cobra.ExactArgs(1),
		Run:  runInstall,
	}

	cmd.Flags().BoolVarP(&installForce, "force", "f", false, "overwrite if exists")
	cmd.Flags().StringVarP(&installVersion, "version", "v", "latest", "specific version (for registry installs)")

	return cmd
}

func runInstall(cmd *cobra.Command, args []string) {
	out := output.New(false)
	input := args[0]

	// Determine if input is a file path or a registry reference
	isRegistryRef := false
	if _, err := os.Stat(input); os.IsNotExist(err) {
		// File doesn't exist, assume it's a registry reference
		isRegistryRef = true
	}

	if isRegistryRef {
		installFromRegistry(input, out)
	} else {
		installFromFile(input, out)
	}
}

func installFromRegistry(input string, out *output.Output) {
	// Parse schema reference
	ref, err := schema.ParseSchemaRef(input)
	if err != nil {
		out.Error("Invalid schema reference: %v", err)
		os.Exit(1)
	}

	// If version flag is set, override the version
	if installVersion != "latest" {
		ref.Version = installVersion
	}

	// Fetch from registry
	registry := schema.NewRegistry()

	fmt.Printf("Fetching %s... ", ref.String())
	s, err := registry.Fetch(ref)
	if err != nil {
		fmt.Println("✗")
		out.Error("Failed to fetch schema: %v", err)
		os.Exit(1)
	}
	fmt.Println("✓")

	// Convert to JSON Schema
	jsonSchema, err := schema.Convert(s, ref.Namespace, "")
	if err != nil {
		out.Error("Failed to convert schema: %v", err)
		os.Exit(1)
	}

	// Initialize storage
	storage, err := schema.NewStorage()
	if err != nil {
		out.Error("Failed to initialize storage: %v", err)
		os.Exit(1)
	}

	// Check if already exists
	if !installForce && storage.Exists(ref.Namespace, ref.Name, s.Version) {
		out.Error("Schema @%s/%s@%s already exists. Use --force to overwrite.", ref.Namespace, ref.Name, s.Version)
		os.Exit(1)
	}

	// Save schema
	if err := storage.Save(ref.Namespace, ref.Name, s.Version, s, jsonSchema); err != nil {
		out.Error("Failed to install schema: %v", err)
		os.Exit(1)
	}

	out.Success("Installed @%s/%s@%s", ref.Namespace, ref.Name, s.Version)
}

func installFromFile(filePath string, out *output.Output) {
	// Parse schema
	s, err := schema.Parse(filePath)
	if err != nil {
		out.Error("Failed to parse schema: %v", err)
		os.Exit(1)
	}

	// Validate schema
	result := schema.Validate(s, false)
	if !result.Valid {
		out.Error("Schema validation failed")
		for _, e := range result.Errors {
			if e.Field != "" {
				fmt.Printf("  ✗ %s: %s\n", e.Field, e.Message)
			} else {
				fmt.Printf("  ✗ %s\n", e.Message)
			}
		}
		os.Exit(1)
	}

	// Extract namespace from filename or use "local"
	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]

	// If the schema has a different name, use it
	if s.Name != "" && s.Name != name {
		name = s.Name
	}

	namespace := "local"

	// Convert to JSON Schema
	jsonSchema, err := schema.Convert(s, namespace, "")
	if err != nil {
		out.Error("Failed to convert schema: %v", err)
		os.Exit(1)
	}

	// Initialize storage
	storage, err := schema.NewStorage()
	if err != nil {
		out.Error("Failed to initialize storage: %v", err)
		os.Exit(1)
	}

	// Check if already exists
	if !installForce && storage.Exists(namespace, name, s.Version) {
		out.Error("Schema @%s/%s@%s already exists. Use --force to overwrite.", namespace, name, s.Version)
		os.Exit(1)
	}

	// Save schema
	if err := storage.Save(namespace, name, s.Version, s, jsonSchema); err != nil {
		out.Error("Failed to install schema: %v", err)
		os.Exit(1)
	}

	out.Success("Installed @%s/%s@%s", namespace, name, s.Version)
	if len(result.Warnings) > 0 {
		fmt.Println()
		for _, w := range result.Warnings {
			if w.Field != "" {
				fmt.Printf("  ⚠ %s: %s\n", w.Field, w.Message)
			} else {
				fmt.Printf("  ⚠ %s\n", w.Message)
			}
		}
	}
}
