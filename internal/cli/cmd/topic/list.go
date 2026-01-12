package topic

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all topic-schema bindings",
	Long:  "Display all configured topic-schema bindings.",
	Run:   runList,
}

var (
	listJSON bool
)

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
}

func runList(cmd *cobra.Command, args []string) {
	store, err := schema.NewBindingStore()
	if err != nil {
		fmt.Printf("✗ Failed to initialize binding store: %v\n", err)
		os.Exit(1)
	}

	bindings := store.List()

	if len(bindings) == 0 {
		fmt.Println("No topic bindings configured")
		return
	}

	if listJSON {
		data, err := json.MarshalIndent(bindings, "", "  ")
		if err != nil {
			fmt.Printf("✗ Failed to marshal bindings: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	// Print as table
	fmt.Printf("%-30s %s\n", "TOPIC PATTERN", "SCHEMA")
	for _, b := range bindings {
		schema := fmt.Sprintf("@%s/%s@%s", b.Namespace, b.Name, b.Version)
		fmt.Printf("%-30s %s\n", b.TopicPattern, schema)
	}
}
