package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/spf13/cobra"
)

var (
	listJSON bool
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed schemas",
		Long: `List all locally installed schemas.

Examples:
  # List all installed schemas
  notif schema list

  # List with JSON output
  notif schema list --json`,
		Run: runList,
	}

	cmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) {
	out := output.New(listJSON)

	// Initialize storage
	storage, err := schema.NewStorage()
	if err != nil {
		out.Error("Failed to initialize storage: %v", err)
		os.Exit(1)
	}

	// List schemas
	schemas, err := storage.List()
	if err != nil {
		out.Error("Failed to list schemas: %v", err)
		os.Exit(1)
	}

	// Output
	if listJSON {
		data, _ := json.MarshalIndent(schemas, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Text output
	if len(schemas) == 0 {
		fmt.Println("No schemas installed")
		fmt.Println()
		fmt.Println("Install a schema:")
		fmt.Println("  notif schema init --name=myschema")
		fmt.Println("  notif schema install myschema.yaml")
		return
	}

	// Print header
	fmt.Printf("%-30s %-10s %-15s %s\n", "NAMESPACE/NAME", "VERSION", "INSTALLED", "DESCRIPTION")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")

	// Print schemas
	for _, s := range schemas {
		ref := fmt.Sprintf("@%s/%s", s.Namespace, s.Name)
		installed := formatTime(s.InstalledAt)
		description := s.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		fmt.Printf("%-30s %-10s %-15s %s\n", ref, s.Version, installed, description)
	}

	fmt.Println()
	fmt.Printf("Total: %d schema(s)\n", len(schemas))
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
