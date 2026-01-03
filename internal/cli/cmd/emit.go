package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
)

var (
	replyTo      string
	replyFilter  string
	replyTimeout time.Duration
)

var emitCmd = &cobra.Command{
	Use:   "emit <topic> [data]",
	Short: "Emit an event to a topic",
	Long: `Emit an event to a topic. Data can be provided as an argument or via stdin.

Examples:
  notif emit orders.created '{"id": 123}'
  echo '{"id": 123}' | notif emit orders.created
  cat event.json | notif emit orders.created

Request-response mode (wait for reply):
  notif emit orders.create '{"id": 123}' \
    --reply-to 'orders.created,orders.failed' \
    --filter '.id == 123' \
    --timeout 30s`,
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

		// Request-response mode
		if replyTo != "" {
			runRequestResponse(c, topic, json.RawMessage(data))
			return
		}

		// Fire-and-forget mode (default)
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

func runRequestResponse(c *client.Client, topic string, data json.RawMessage) {
	// Parse reply topics
	topics := strings.Split(replyTo, ",")
	for i := range topics {
		topics[i] = strings.TrimSpace(topics[i])
	}

	// Parse jq filter if provided
	var jqCode *gojq.Code
	if replyFilter != "" {
		query, err := gojq.Parse(replyFilter)
		if err != nil {
			out.Error("Invalid jq filter: %v", err)
			os.Exit(1)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			out.Error("Failed to compile jq filter: %v", err)
			os.Exit(1)
		}
		jqCode = code
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), replyTimeout)
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to reply topics first
	sub, err := c.Subscribe(ctx, topics, client.SubscribeOptions{From: "latest"})
	if err != nil {
		out.Error("Failed to subscribe: %v", err)
		os.Exit(1)
	}
	defer sub.Close()

	// Wait a bit for WebSocket connection to establish
	time.Sleep(50 * time.Millisecond)

	// Emit the request
	resp, err := c.Emit(topic, data)
	if err != nil {
		out.Error("Failed to emit: %v", err)
		os.Exit(1)
	}

	if !jsonOutput {
		out.Success("Event emitted, waiting for response...")
		out.KeyValue("ID", resp.ID)
		out.KeyValue("Listening", strings.Join(topics, ", "))
	}

	// Wait for matching response
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				out.Error("Subscription closed")
				os.Exit(1)
			}

			// Check if event matches filter
			if matchesJqFilter(jqCode, event.Data) {
				out.Event(event.ID, event.Topic, event.Data, event.Timestamp)
				return
			}

		case err := <-sub.Errors():
			out.Error("Subscription error: %v", err)
			os.Exit(1)

		case <-sigCh:
			if !jsonOutput {
				out.Info("Interrupted")
			}
			os.Exit(130)

		case <-ctx.Done():
			out.Error("Timeout waiting for response")
			os.Exit(1)
		}
	}
}

func matchesJqFilter(code *gojq.Code, data json.RawMessage) bool {
	if code == nil {
		return true // no filter = match any
	}

	var input any
	if err := json.Unmarshal(data, &input); err != nil {
		return false
	}

	iter := code.Run(input)
	v, ok := iter.Next()
	if !ok {
		return false
	}

	// Handle error from jq
	if err, ok := v.(error); ok {
		_ = err
		return false
	}

	// jq filter expressions return true/false
	if b, ok := v.(bool); ok {
		return b
	}

	// Non-nil result means match (for select-style filters)
	return v != nil
}

func init() {
	emitCmd.Flags().StringVar(&replyTo, "reply-to", "", "topics to wait for response (comma-separated)")
	emitCmd.Flags().StringVar(&replyFilter, "filter", "", "jq expression to match response")
	emitCmd.Flags().DurationVar(&replyTimeout, "timeout", 30*time.Second, "timeout waiting for response")
	rootCmd.AddCommand(emitCmd)
}
