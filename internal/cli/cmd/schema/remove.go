package schema

import (
	"fmt"
	"os"
	"strings"

	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/spf13/cobra"
)

var (
	removeAll bool
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <namespace/name>",
		Short: "Remove a locally installed schema",
		Long: `Remove a locally installed schema.

By default, removes only the latest version. Use --all to remove all versions.

Examples:
  # Remove latest version
  notif schema remove @local/agent

  # Remove specific version
  notif schema remove @local/agent@1.0.0

  # Remove all versions
  notif schema remove @local/agent --all`,
		Args: cobra.ExactArgs(1),
		Run:  runRemove,
	}

	cmd.Flags().BoolVar(&removeAll, "all", false, "remove all versions")

	return cmd
}

func runRemove(cmd *cobra.Command, args []string) {
	out := output.New(false)
	refStr := args[0]

	// Parse schema reference
	ref, err := schema.ParseSchemaRef(refStr)
	if err != nil {
		out.Error("Invalid schema reference: %v", err)
		os.Exit(1)
	}

	namespace := ref.Namespace
	name := ref.Name
	version := ref.Version

	// Initialize storage
	storage, err := schema.NewStorage()
	if err != nil {
		out.Error("Failed to initialize storage: %v", err)
		os.Exit(1)
	}

	// Remove all versions
	if removeAll {
		if version != "latest" && version != "" {
			out.Error("Cannot specify version with --all flag")
			os.Exit(1)
		}

		// Check if schema exists
		versions, err := storage.ListVersions(namespace, name)
		if err != nil {
			out.Error("Failed to list versions: %v", err)
			os.Exit(1)
		}

		if len(versions) == 0 {
			out.Error("Schema @%s/%s not found", namespace, name)
			os.Exit(1)
		}

		// Remove all
		if err := storage.RemoveAll(namespace, name); err != nil {
			out.Error("Failed to remove schema: %v", err)
			os.Exit(1)
		}

		out.Success("Removed all versions of @%s/%s (%d version(s))", namespace, name, len(versions))
		return
	}

	// Remove specific version or latest
	if version == "" {
		// Find latest version
		versions, err := storage.ListVersions(namespace, name)
		if err != nil {
			out.Error("Failed to list versions: %v", err)
			os.Exit(1)
		}

		if len(versions) == 0 {
			out.Error("Schema @%s/%s not found", namespace, name)
			os.Exit(1)
		}

		// Use the first version (they should be sorted)
		version = versions[0]
	}

	// Check if exists
	if !storage.Exists(namespace, name, version) {
		out.Error("Schema @%s/%s@%s not found", namespace, name, version)

		// Try to suggest available versions
		versions, _ := storage.ListVersions(namespace, name)
		if len(versions) > 0 {
			fmt.Println()
			fmt.Println("Available versions:")
			for _, v := range versions {
				fmt.Printf("  - %s\n", v)
			}
		}
		os.Exit(1)
	}

	// Remove
	if err := storage.Remove(namespace, name, version); err != nil {
		out.Error("Failed to remove schema: %v", err)
		os.Exit(1)
	}

	out.Success("Removed @%s/%s@%s", namespace, name, version)

	// Show remaining versions if any
	versions, _ := storage.ListVersions(namespace, name)
	if len(versions) > 0 {
		fmt.Println()
		fmt.Printf("Remaining versions (%d):\n", len(versions))
		for _, v := range versions {
			fmt.Printf("  - %s\n", v)
		}
	}
}

// normalizeRef normalizes a schema reference.
func normalizeRef(ref string) string {
	// Ensure @ prefix
	if !strings.HasPrefix(ref, "@") {
		ref = "@" + ref
	}
	return ref
}
