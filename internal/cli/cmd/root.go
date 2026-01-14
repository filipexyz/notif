package cmd

import (
	"fmt"
	"os"

	"github.com/filipexyz/notif/internal/cli/config"
	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	serverURL  string
	jsonOutput bool
	cfg        *config.Config
	out        *output.Output
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "notif",
	Short: "CLI for notif.sh event hub",
	Long:  `notif is a command-line tool for interacting with the notif.sh managed pub/sub event hub.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		out = output.New(jsonOutput)

		// Load config (ignore errors for commands that don't need it)
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			cfg = &config.Config{}
		}

		// Auth token priority: NOTIF_JWT > NOTIF_API_KEY > config
		if jwt := os.Getenv("NOTIF_JWT"); jwt != "" {
			cfg.APIKey = jwt // JWT works as bearer token too
		} else if apiKey := os.Getenv("NOTIF_API_KEY"); apiKey != "" {
			cfg.APIKey = apiKey
		}

		// Server URL priority: flag > env > config > default
		if serverURL == "" {
			if envServer := os.Getenv("NOTIF_SERVER"); envServer != "" {
				serverURL = envServer
			} else if cfg.Server != "" {
				serverURL = cfg.Server
			} else {
				serverURL = client.DefaultServer
			}
		}
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.notif/config.json)")
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "server URL")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
}

// getClient creates a client with current config.
func getClient() *client.Client {
	apiKey := cfg.APIKey
	return client.New(apiKey, client.WithServer(serverURL))
}
