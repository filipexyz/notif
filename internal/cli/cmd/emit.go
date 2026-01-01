package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var emitCmd = &cobra.Command{
	Use:   "emit <topic> [data]",
	Short: "Emit an event to a topic",
	Long: `Emit an event to a topic. Data can be provided as an argument or via stdin.

Examples:
  notif emit orders.created '{"id": 123}'
  echo '{"id": 123}' | notif emit orders.created
  cat event.json | notif emit orders.created`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		topic := args[0]

		// Get data from arg or stdin
		var data string
		if len(args) > 1 {
			data = args[1]
		} else {
			// Check if stdin has data
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				scanner := bufio.NewScanner(os.Stdin)
				var lines []string
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}
				data = strings.Join(lines, "\n")
			}
		}

		if data == "" {
			out.Error("No data provided. Pass as argument or pipe via stdin.")
			return
		}

		// Validate JSON
		if !json.Valid([]byte(data)) {
			out.Error("Invalid JSON data")
			return
		}

		c := getClient()
		resp, err := c.Emit(topic, json.RawMessage(data))
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{
					"error": err.Error(),
				})
			} else {
				out.Error("Failed to emit: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(resp)
			return
		}

		out.Success("Event emitted")
		out.KeyValue("ID", resp.ID)
		out.KeyValue("Topic", resp.Topic)
		out.KeyValue("Created", resp.CreatedAt.Format("2006-01-02 15:04:05"))
	},
}

func init() {
	rootCmd.AddCommand(emitCmd)
}
