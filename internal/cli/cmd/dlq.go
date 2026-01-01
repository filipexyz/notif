package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
)

var dlqCmd = &cobra.Command{
	Use:   "dlq",
	Short: "Manage dead letter queue",
	Long:  `View, replay, and delete messages from the dead letter queue.`,
}

var dlqListTopic string
var dlqListLimit int

var dlqListCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages in the DLQ",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.DLQList(dlqListTopic, dlqListLimit)
		if err != nil {
			out.Error("Failed to list DLQ: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No messages in DLQ")
			return
		}

		out.Header("Dead Letter Queue")
		out.KeyValue("Count", strconv.Itoa(result.Count))
		out.Divider()

		for _, entry := range result.Messages {
			out.Info("Seq: %d", entry.Seq)
			out.KeyValue("ID", entry.Message.ID)
			out.KeyValue("Topic", entry.Message.OriginalTopic)
			out.KeyValue("Attempts", strconv.Itoa(entry.Message.Attempts))
			out.KeyValue("Failed At", entry.Message.FailedAt.Format("2006-01-02 15:04:05"))
			if entry.Message.LastError != "" {
				out.KeyValue("Error", entry.Message.LastError)
			}
			if entry.Message.ConsumerGroup != "" {
				out.KeyValue("Group", entry.Message.ConsumerGroup)
			}
			out.Divider()
		}
	},
}

var dlqReplayCmd = &cobra.Command{
	Use:   "replay <seq>",
	Short: "Replay a DLQ message to its original topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		seq, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			out.Error("Invalid sequence number")
			return
		}

		c := getClient()
		if err := c.DLQReplay(seq); err != nil {
			out.Error("Failed to replay: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(map[string]string{"status": "replayed"})
			return
		}

		out.Success("Message replayed to original topic")
	},
}

var dlqDeleteCmd = &cobra.Command{
	Use:   "delete <seq>",
	Short: "Delete a message from the DLQ",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		seq, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			out.Error("Invalid sequence number")
			return
		}

		c := getClient()
		if err := c.DLQDelete(seq); err != nil {
			out.Error("Failed to delete: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(map[string]string{"status": "deleted"})
			return
		}

		out.Success("Message deleted from DLQ")
	},
}

var dlqReplayAllTopic string

var dlqReplayAllCmd = &cobra.Command{
	Use:   "replay-all",
	Short: "Replay all messages from the DLQ",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.DLQReplayAll(dlqReplayAllTopic)
		if err != nil {
			out.Error("Failed to replay all: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		out.Success("Replayed %d messages (%d failed)", result.Replayed, result.Failed)
	},
}

var dlqPurgeTopic string

var dlqPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete all messages from the DLQ",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.DLQPurge(dlqPurgeTopic)
		if err != nil {
			out.Error("Failed to purge: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		out.Success("Deleted %d messages from DLQ", result.Deleted)
	},
}

func init() {
	dlqListCmd.Flags().StringVar(&dlqListTopic, "topic", "", "filter by topic")
	dlqListCmd.Flags().IntVar(&dlqListLimit, "limit", 100, "max messages to list")

	dlqReplayAllCmd.Flags().StringVar(&dlqReplayAllTopic, "topic", "", "filter by topic")
	dlqPurgeCmd.Flags().StringVar(&dlqPurgeTopic, "topic", "", "filter by topic")

	dlqCmd.AddCommand(dlqListCmd)
	dlqCmd.AddCommand(dlqReplayCmd)
	dlqCmd.AddCommand(dlqDeleteCmd)
	dlqCmd.AddCommand(dlqReplayAllCmd)
	dlqCmd.AddCommand(dlqPurgeCmd)

	rootCmd.AddCommand(dlqCmd)
}
