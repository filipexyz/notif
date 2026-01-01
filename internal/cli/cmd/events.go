package cmd

import (
	"strconv"
	"time"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Query historical events",
	Long:  `List and retrieve historical events from the stream.`,
}

var (
	eventsListTopic string
	eventsListFrom  string
	eventsListTo    string
	eventsListLimit int
)

var eventsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List historical events",
	Long: `List historical events with optional filters.

Examples:
  notif events list
  notif events list --topic orders.created
  notif events list --topic "orders.*" --from 2024-01-01T00:00:00Z
  notif events list --limit 50`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		opts := client.EventsQueryOptions{
			Topic: eventsListTopic,
			Limit: eventsListLimit,
		}

		if eventsListFrom != "" {
			if t, err := time.Parse(time.RFC3339, eventsListFrom); err == nil {
				opts.From = t
			} else if d, err := time.ParseDuration(eventsListFrom); err == nil {
				opts.From = time.Now().Add(-d)
			}
		}

		if eventsListTo != "" {
			if t, err := time.Parse(time.RFC3339, eventsListTo); err == nil {
				opts.To = t
			}
		}

		c := getClient()
		result, err := c.EventsList(opts)
		if err != nil {
			out.Error("Failed to list events: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No events found")
			return
		}

		out.Header("Events")
		out.KeyValue("Count", strconv.Itoa(result.Count))
		out.Divider()

		for _, e := range result.Events {
			out.Event(e.Event.ID, e.Event.Topic, e.Event.Data, e.Event.Timestamp)
		}
	},
}

var eventsGetCmd = &cobra.Command{
	Use:   "get <seq>",
	Short: "Get a specific event by sequence number",
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
		event, err := c.EventsGet(seq)
		if err != nil {
			out.Error("Failed to get event: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(event)
			return
		}

		out.Header("Event")
		out.KeyValue("Seq", strconv.FormatUint(event.Seq, 10))
		out.KeyValue("ID", event.Event.ID)
		out.KeyValue("Topic", event.Event.Topic)
		out.KeyValue("Timestamp", event.Event.Timestamp.Format("2006-01-02 15:04:05"))
		out.Divider()
		out.Info("Data: %s", string(event.Event.Data))
	},
}

var eventsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show stream statistics",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		stats, err := c.EventsStats()
		if err != nil {
			out.Error("Failed to get stats: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(stats)
			return
		}

		out.Header("Stream Statistics")
		out.KeyValue("Messages", strconv.FormatUint(stats.Messages, 10))
		out.KeyValue("Bytes", formatBytes(stats.Bytes))
		out.KeyValue("First Seq", strconv.FormatUint(stats.FirstSeq, 10))
		out.KeyValue("Last Seq", strconv.FormatUint(stats.LastSeq, 10))
		if !stats.FirstTime.IsZero() {
			out.KeyValue("First Time", stats.FirstTime.Format("2006-01-02 15:04:05"))
		}
		if !stats.LastTime.IsZero() {
			out.KeyValue("Last Time", stats.LastTime.Format("2006-01-02 15:04:05"))
		}
		out.KeyValue("Consumers", strconv.Itoa(stats.Consumers))
	},
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatUint(b, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(b)/float64(div), 'f', 1, 64) + " " + string("KMGTPE"[exp]) + "B"
}

func init() {
	eventsListCmd.Flags().StringVar(&eventsListTopic, "topic", "", "filter by topic (supports wildcards)")
	eventsListCmd.Flags().StringVar(&eventsListFrom, "from", "", "start time (RFC3339 or duration like 1h, 24h)")
	eventsListCmd.Flags().StringVar(&eventsListTo, "to", "", "end time (RFC3339)")
	eventsListCmd.Flags().IntVar(&eventsListLimit, "limit", 100, "max events to return")

	eventsCmd.AddCommand(eventsListCmd)
	eventsCmd.AddCommand(eventsGetCmd)
	eventsCmd.AddCommand(eventsStatsCmd)

	rootCmd.AddCommand(eventsCmd)
}
