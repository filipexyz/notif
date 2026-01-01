package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set at build time via ldflags
var (
	Version = "dev"
	Commit  = "unknown"
)

// VersionString returns the formatted version string
func VersionString() string {
	if Version == "dev" {
		return fmt.Sprintf("notif dev (%s)", Commit)
	}
	return fmt.Sprintf("notif v%s", Version)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(VersionString())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = VersionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}
