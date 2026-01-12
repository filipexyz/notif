package topic

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var bindCmd = &cobra.Command{
	Use:   "bind <topic-pattern> <namespace/name>",
	Short: "Bind a schema to a topic pattern",
	Long: `Bind a schema to a topic pattern for automatic validation.

Topic patterns support wildcards:
  * - matches exactly one level
  # - matches one or more levels

Examples:
  # Bind exact topic
  notif topic bind 'agents.online' @filipelabs/agent

  # Bind with wildcard (single level)
  notif topic bind 'agents.*' @filipelabs/agent

  # Bind with wildcard (multiple levels)
  notif topic bind 'agents.#' @filipelabs/agent`,
	Args: cobra.ExactArgs(2),
	Run:  runBind,
}

var (
	bindVersion string
)

func init() {
	bindCmd.Flags().StringVarP(&bindVersion, "version", "v", "latest", "Schema version")
}

func runBind(cmd *cobra.Command, args []string) {
	topicPattern := args[0]
	schemaRef := args[1]

	// Parse schema reference
	ref, err := schema.ParseSchemaRef(schemaRef)
	if err != nil {
		fmt.Printf("✗ Invalid schema reference: %v\n", err)
		os.Exit(1)
	}

	// Override version if specified
	if bindVersion != "latest" {
		ref.Version = bindVersion
	}

	// Check if schema is installed
	storage, err := schema.NewStorage()
	if err != nil {
		fmt.Printf("✗ Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	// Resolve version if "latest"
	version := ref.Version
	if version == "latest" {
		versions, _ := storage.ListVersions(ref.Namespace, ref.Name)
		if len(versions) == 0 {
			fmt.Printf("✗ Schema not installed: @%s/%s\n", ref.Namespace, ref.Name)
			fmt.Printf("  Try: notif schema install @%s/%s\n", ref.Namespace, ref.Name)
			os.Exit(1)
		}
		version = versions[len(versions)-1] // Last version is latest
	}

	// Verify schema exists
	if _, _, _, err := storage.Load(ref.Namespace, ref.Name, version); err != nil {
		fmt.Printf("✗ Schema not installed: @%s/%s@%s\n", ref.Namespace, ref.Name, version)
		fmt.Printf("  Try: notif schema install @%s/%s@%s\n", ref.Namespace, ref.Name, version)
		os.Exit(1)
	}

	// Add binding
	store, err := schema.NewBindingStore()
	if err != nil {
		fmt.Printf("✗ Failed to initialize binding store: %v\n", err)
		os.Exit(1)
	}

	if err := store.Add(topicPattern, ref.Namespace, ref.Name, version); err != nil {
		fmt.Printf("✗ Failed to add binding: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Bound topic pattern '%s' to @%s/%s@%s\n",
		topicPattern, ref.Namespace, ref.Name, version)
	fmt.Println("\nEvents published to matching topics will now be validated against this schema.")
}
