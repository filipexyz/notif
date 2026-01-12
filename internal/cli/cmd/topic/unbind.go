package topic

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var unbindCmd = &cobra.Command{
	Use:   "unbind <topic-pattern>",
	Short: "Remove a topic-schema binding",
	Long: `Remove a topic-schema binding by topic pattern.

Examples:
  # Remove binding
  notif topic unbind 'agents.*'`,
	Args: cobra.ExactArgs(1),
	Run:  runUnbind,
}

func runUnbind(cmd *cobra.Command, args []string) {
	topicPattern := args[0]

	store, err := schema.NewBindingStore()
	if err != nil {
		fmt.Printf("✗ Failed to initialize binding store: %v\n", err)
		os.Exit(1)
	}

	if err := store.Remove(topicPattern); err != nil {
		fmt.Printf("✗ %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Removed binding for '%s'\n", topicPattern)
}
