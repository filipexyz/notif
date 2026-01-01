package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

var (
	subscribeGroup   string
	subscribeFrom    string
	subscribeNoAck   bool
)

var subscribeCmd = &cobra.Command{
	Use:   "subscribe <topics...>",
	Short: "Subscribe to topics",
	Long: `Subscribe to one or more topics and receive events in real-time.

Examples:
  notif subscribe orders.created
  notif subscribe "orders.*"
  notif subscribe orders.created users.signup
  notif subscribe --group processor "orders.*"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		topics := args

		c := getClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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
			out.Info("Waiting for events... (Ctrl+C to exit)")
			out.Divider()
		}

		// Handle signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					return
				}
				out.Event(event.ID, event.Topic, event.Data, event.Timestamp)

			case err := <-sub.Errors():
				out.Error("Subscription error: %v", err)
				return

			case <-sigCh:
				if !jsonOutput {
					out.Info("Disconnecting...")
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
	rootCmd.AddCommand(subscribeCmd)
}
