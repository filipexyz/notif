package schema

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search available schemas in the registry",
	Long:  "Search for schemas in the public registry by name, namespace, or description.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSearch,
}

var (
	searchNamespace string
	searchLimit     int
	searchJSON      bool
)

func init() {
	searchCmd.Flags().StringVar(&searchNamespace, "namespace", "", "Filter by namespace")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output in JSON format")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	registry := schema.NewRegistry()

	opts := schema.SearchOptions{
		Namespace: searchNamespace,
		Limit:     searchLimit,
	}

	results, err := registry.Search(query, opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No schemas found")
		return nil
	}

	if searchJSON {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	fmt.Printf("%-30s %-10s %s\n", "NAMESPACE/NAME", "LATEST", "DESCRIPTION")
	for _, s := range results {
		name := fmt.Sprintf("@%s/%s", s.Namespace, s.Name)
		desc := s.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Printf("%-30s %-10s %s\n", name, s.Version, desc)
	}

	return nil
}
