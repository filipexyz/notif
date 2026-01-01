package cmd

import (
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server health",
	Long:  `Check the health status of the notif.sh server.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()

		health, err := c.Health()
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{
					"status": "error",
					"error":  err.Error(),
				})
			} else {
				out.Error("Server unreachable: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(health)
			return
		}

		out.Success("Server is healthy")
		out.KeyValue("Status", health.Status)
		if health.Version != "" {
			out.KeyValue("Version", health.Version)
		}
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
