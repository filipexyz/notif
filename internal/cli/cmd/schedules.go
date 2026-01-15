package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scheduleStatus string

var schedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "Manage scheduled events",
	Long:  `List, view, cancel, and run scheduled events.`,
}

var schedulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled events",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		resp, err := c.ListSchedules(scheduleStatus, 50, 0)
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{"error": err.Error()})
			} else {
				out.Error("Failed to list schedules: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(resp)
			return
		}

		if resp.Count == 0 {
			out.Info("No scheduled events found")
			return
		}

		out.Header("Scheduled Events")
		fmt.Println()

		for _, s := range resp.Schedules {
			statusColor := ""
			switch s.Status {
			case "pending":
				statusColor = "\033[33m" // yellow
			case "completed":
				statusColor = "\033[32m" // green
			case "cancelled":
				statusColor = "\033[90m" // gray
			case "failed":
				statusColor = "\033[31m" // red
			}

			fmt.Printf("  %s%-12s%s %-20s %s%s%s  %s\n",
				"\033[36m", s.ID, "\033[0m",
				truncate(s.Topic, 20),
				statusColor, s.Status, "\033[0m",
				s.ScheduledFor.Format("2006-01-02 15:04:05"),
			)
		}
		fmt.Println()
	},
}

var schedulesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get scheduled event details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		s, err := c.GetSchedule(args[0])
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{"error": err.Error()})
			} else {
				out.Error("Failed to get schedule: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(s)
			return
		}

		out.Header("Scheduled Event")
		out.KeyValue("ID", s.ID)
		out.KeyValue("Topic", s.Topic)
		out.KeyValue("Status", s.Status)
		out.KeyValue("Scheduled For", s.ScheduledFor.Format("2006-01-02 15:04:05 MST"))
		out.KeyValue("Created", s.CreatedAt.Format("2006-01-02 15:04:05 MST"))
		if s.ExecutedAt != nil {
			out.KeyValue("Executed", s.ExecutedAt.Format("2006-01-02 15:04:05 MST"))
		}
		if s.Error != nil {
			out.KeyValue("Error", *s.Error)
		}
		fmt.Println()
		fmt.Printf("  Data: %s\n", string(s.Data))
	},
}

var schedulesCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a pending scheduled event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		err := c.CancelSchedule(args[0])
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{"error": err.Error()})
			} else {
				out.Error("Failed to cancel schedule: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(map[string]string{"status": "cancelled"})
			return
		}

		out.Success("Schedule cancelled: %s", args[0])
	},
}

var schedulesRunCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Execute a scheduled event immediately",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		resp, err := c.RunSchedule(args[0])
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{"error": err.Error()})
			} else {
				out.Error("Failed to run schedule: %v", err)
			}
			return
		}

		if jsonOutput {
			out.JSON(resp)
			return
		}

		out.Success("Schedule executed")
		out.KeyValue("Schedule ID", resp.ScheduleID)
		out.KeyValue("Event ID", resp.EventID)
	},
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	schedulesListCmd.Flags().StringVar(&scheduleStatus, "status", "", "filter by status (pending, completed, cancelled, failed)")

	schedulesCmd.AddCommand(schedulesListCmd)
	schedulesCmd.AddCommand(schedulesGetCmd)
	schedulesCmd.AddCommand(schedulesCancelCmd)
	schedulesCmd.AddCommand(schedulesRunCmd)
	rootCmd.AddCommand(schedulesCmd)
}
