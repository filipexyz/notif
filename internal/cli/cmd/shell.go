package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Commands blocked in shell mode for security
var blockedCommands = map[string]bool{
	"auth":   true, // Don't allow saving credentials to server filesystem
	"config": true, // Don't allow modifying server config
}

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Interactive shell mode",
	Long:  `Start an interactive shell for running notif commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("notif> ")
			if !scanner.Scan() {
				break
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if line == "exit" || line == "quit" {
				break
			}

			// Parse command and execute
			cmdArgs := parseArgs(line)
			if len(cmdArgs) == 0 {
				continue
			}

			// Block dangerous commands
			if blockedCommands[cmdArgs[0]] {
				out.Error("command '%s' is not available in shell mode", cmdArgs[0])
				fmt.Println()
				continue
			}

			// Create a fresh root command for each execution
			// to avoid state pollution between commands
			rootCmd.SetArgs(cmdArgs)
			rootCmd.Execute()
			fmt.Println()
		}
	},
}

// parseArgs splits a command line into arguments, respecting quotes
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range line {
		switch {
		case (r == '"' || r == '\'') && !inQuote:
			inQuote = true
			quoteChar = r
		case r == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
