package cmd

import (
	"regexp"

	"github.com/filipexyz/notif/internal/cli/config"
	"github.com/spf13/cobra"
)

var apiKeyRegex = regexp.MustCompile(`^nsh_[a-zA-Z0-9]{20}$`)

var authCmd = &cobra.Command{
	Use:   "auth <api-key>",
	Short: "Authenticate with an API key",
	Long:  `Save your API key to the config file for future requests.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		apiKey := args[0]

		// Validate format
		if !apiKeyRegex.MatchString(apiKey) {
			out.Error("Invalid API key format. Expected: nsh_<20 chars>")
			return
		}

		// Save to config
		cfg.APIKey = apiKey
		if serverURL != "" {
			cfg.Server = serverURL
		}

		if err := config.Save(cfg, cfgFile); err != nil {
			out.Error("Failed to save config: %v", err)
			return
		}

		out.Success("API key saved")
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}
