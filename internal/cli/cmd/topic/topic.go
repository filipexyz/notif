package topic

import (
	"github.com/spf13/cobra"
)

// NewTopicCmd returns the topic command
func NewTopicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Manage topic-schema bindings",
		Long: `Bind schemas to topics for automatic validation.

When a topic is bound to a schema, all events published to that topic
will be validated against the schema.

Examples:
  # Bind a topic to a schema
  notif topic bind 'agents.*' @filipelabs/agent

  # List all bindings
  notif topic list

  # Remove a binding
  notif topic unbind 'agents.*'`,
	}

	cmd.AddCommand(bindCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(unbindCmd)

	return cmd
}
