package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
)

var (
	subscribeGroup   string
	subscribeFrom    string
	subscribeNoAck   bool
	subscribeFilter  string
	subscribeOnce    bool
	subscribeCount   int
	subscribeTimeout time.Duration
)

var subscribeCmd = &cobra.Command{
	Use:   "subscribe <topics...>",
	Short: "Subscribe to topics",
	Long: `Subscribe to one or more topics and receive events in real-time.

Examples:
  notif subscribe orders.created
  notif subscribe "orders.*"
  notif subscribe orders.created users.signup
  notif subscribe --group processor "orders.*"

Filter and auto-exit:
  notif subscribe 'orders.*' --filter '.status == "completed"' --once
  notif subscribe 'orders.*' --filter '.amount > 100' --count 5 --timeout 30s`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		topics := args

		// Parse jq filter if provided
		var jqCode *gojq.Code
		if subscribeFilter != "" {
			code, err := compileJqFilter(subscribeFilter)
			if err != nil {
				out.Error("Invalid jq filter: %v", err)
				os.Exit(1)
			}
			jqCode = code
		}

		// Normalize --once to --count 1
		if subscribeOnce {
			subscribeCount = 1
		}

		c := getClient()

		// Set up context with optional timeout
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if subscribeTimeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), subscribeTimeout)
			defer cancel()
		}

		opts := client.SubscribeOptions{
			AutoAck: !subscribeNoAck,
			Group:   subscribeGroup,
			From:    subscribeFrom,
		}

		sub, err := c.Subscribe(ctx, topics, opts)
		if err != nil {
			out.Error("Failed to subscribe: %v", err)
			return
		}
		defer sub.Close()

		if !jsonOutput {
			out.Success("Subscribed to %v", topics)
			if subscribeGroup != "" {
				out.KeyValue("Group", subscribeGroup)
			}
			if subscribeFilter != "" {
				out.KeyValue("Filter", subscribeFilter)
			}
			if subscribeCount > 0 {
				out.KeyValue("Exit after", fmt.Sprintf("%d events", subscribeCount))
			}
			out.Info("Waiting for events... (Ctrl+C to exit)")
			out.Divider()
		}

		// Handle signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		matchCount := 0

		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					return
				}

				// Check filter (no $input for subscribe)
				if !matchesJqFilter(jqCode, event.Data, nil) {
					continue // skip non-matching events
				}

				out.Event(event.ID, event.Topic, event.Data, event.Timestamp)
				matchCount++

				// Check exit condition
				if subscribeCount > 0 && matchCount >= subscribeCount {
					return
				}

			case err := <-sub.Errors():
				// Log error but don't exit - SDK will auto-reconnect
				if _, ok := err.(*client.ReconnectedError); ok {
					out.Success("Reconnected")
				} else {
					out.Warn("Connection error: %v (reconnecting...)", err)
				}

			case <-sigCh:
				if !jsonOutput {
					out.Info("Disconnecting...")
				}
				return

			case <-ctx.Done():
				if subscribeTimeout > 0 {
					out.Error("Timeout waiting for events")
					os.Exit(1)
				}
				return
			}
		}
	},
}

func init() {
	subscribeCmd.Flags().StringVar(&subscribeGroup, "group", "", "consumer group name")
	subscribeCmd.Flags().StringVar(&subscribeFrom, "from", "latest", "start position (latest, beginning)")
	subscribeCmd.Flags().BoolVar(&subscribeNoAck, "no-auto-ack", false, "disable automatic acknowledgment")
	subscribeCmd.Flags().StringVar(&subscribeFilter, "filter", "", "jq expression to filter events")
	subscribeCmd.Flags().BoolVar(&subscribeOnce, "once", false, "exit after first matching event")
	subscribeCmd.Flags().IntVar(&subscribeCount, "count", 0, "exit after N matching events")
	subscribeCmd.Flags().DurationVar(&subscribeTimeout, "timeout", 0, "timeout waiting for events")
	rootCmd.AddCommand(subscribeCmd)
}
