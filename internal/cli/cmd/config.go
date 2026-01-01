package cmd

import (
	"github.com/filipexyz/notif/internal/cli/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		path := cfgFile
		if path == "" {
			path = config.DefaultPath()
		}

		if jsonOutput {
			out.JSON(map[string]any{
				"path":        path,
				"api_key":     maskAPIKey(cfg.APIKey),
				"server":      serverURL,
				"environment": cfg.Environment,
			})
			return
		}

		out.Header("Configuration")
		out.KeyValue("Path", path)
		out.KeyValue("API Key", maskAPIKey(cfg.APIKey))
		out.KeyValue("Server", serverURL)
		out.KeyValue("Environment", cfg.Environment)
	},
}

func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) < 16 {
		return "***"
	}
	return key[:12] + "..." + key[len(key)-4:]
}

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
