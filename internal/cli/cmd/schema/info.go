package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/filipexyz/notif/internal/schema"
)

var infoCmd = &cobra.Command{
	Use:   "info <namespace/name>",
	Short: "Show detailed information about a schema",
	Long:  "Display details, versions, and fields for a schema from the registry.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

var (
	infoVersion string
	infoJSON    bool
)

func init() {
	infoCmd.Flags().StringVarP(&infoVersion, "version", "v", "latest", "Show specific version")
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output in JSON format")
}

func runInfo(cmd *cobra.Command, args []string) error {
	ref, err := schema.ParseSchemaRef(args[0])
	if err != nil {
		return err
	}

	registry := schema.NewRegistry()

	info, err := registry.GetSchemaInfo(ref)
	if err != nil {
		return fmt.Errorf("failed to get schema info: %w", err)
	}

	if infoJSON {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal info: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print human-readable format
	fmt.Printf("@%s/%s\n\n", info.Namespace, info.Name)

	if info.Description != "" {
		fmt.Printf("Description: %s\n", info.Description)
	}

	fmt.Printf("Latest: %s\n\n", info.Latest)

	// Print versions
	fmt.Println("Versions:")
	for i, v := range info.Versions {
		marker := " "
		if v == info.Latest {
			marker = "*"
		}
		fmt.Printf("  %s %s", marker, v)
		if i == 0 || v == info.Latest {
			fmt.Print(" (latest)")
		}
		fmt.Println()
	}
	fmt.Println()

	// Print fields
	if len(info.Schema.Fields) > 0 {
		fmt.Println("Fields:")
		for name, field := range info.Schema.Fields {
			req := ""
			if field.Required {
				req = " (required)"
			}
			fmt.Printf("  %-20s %-15s%s", name, field.Type, req)
			if field.Description != "" {
				fmt.Printf(" - %s", field.Description)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Print install command
	fmt.Println("Install:")
	fmt.Printf("  notif schema install @%s/%s\n", info.Namespace, info.Name)

	// Print README if available
	if info.README != "" {
		fmt.Println("\n" + strings.Repeat("-", 60))
		fmt.Println(info.README)
	}

	return nil
}
