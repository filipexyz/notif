package schema

import (
	"github.com/spf13/cobra"
)

// NewSchemaCmd returns the schema command.
func NewSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage event schemas",
		Long: `Manage event schemas for notif.sh.

Schemas define the structure of your events and enable validation,
type generation, and documentation.

Examples:
  # Create a new schema
  notif schema init --name=agent

  # Validate a schema
  notif schema validate ./schema.yaml

  # Install a schema locally
  notif schema install @filipelabs/agent

  # List installed schemas
  notif schema list`,
	}

	// Add subcommands
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(searchCmd)
	cmd.AddCommand(infoCmd)
	cmd.AddCommand(publishCmd)
	cmd.AddCommand(exportCmd)

	return cmd
}
