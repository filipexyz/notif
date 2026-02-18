package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

var (
	auditOrg    string
	auditAction string
	auditSince  string
	auditLimit  int
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Query the audit log",
	Long: `View audit log entries for security-sensitive operations.

Examples:
  notif audit
  notif audit --org org_1 --since 1h
  notif audit --action event.emit --limit 10
  notif audit --json`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.AuditList(client.AuditQueryOptions{
			Org:    auditOrg,
			Action: auditAction,
			Since:  auditSince,
			Limit:  auditLimit,
		})
		if err != nil {
			out.Error("Failed to query audit log: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result.Entries)
			return
		}

		if result.Count == 0 {
			out.Info("No audit entries found")
			return
		}

		out.Header("Audit Log")
		out.Divider()

		for _, entry := range result.Entries {
			out.Info("[%d] %s  %s", entry.ID, entry.Timestamp, entry.Action)
			out.KeyValue("Actor", entry.Actor)
			if entry.OrgID != "" {
				out.KeyValue("Org", entry.OrgID)
			}
			if entry.Target != "" {
				out.KeyValue("Target", entry.Target)
			}
			if entry.IPAddress != "" {
				out.KeyValue("IP", entry.IPAddress)
			}
			if entry.Detail != nil {
				var pretty map[string]any
				if json.Unmarshal(entry.Detail, &pretty) == nil {
					for k, v := range pretty {
						out.KeyValue(fmt.Sprintf("  %s", k), fmt.Sprintf("%v", v))
					}
				}
			}
			out.Divider()
		}
	},
}

func init() {
	auditCmd.Flags().StringVar(&auditOrg, "org", "", "filter by organization ID")
	auditCmd.Flags().StringVar(&auditAction, "action", "", "filter by action (e.g. event.emit)")
	auditCmd.Flags().StringVar(&auditSince, "since", "", "filter events since duration (e.g. 1h, 30m)")
	auditCmd.Flags().IntVar(&auditLimit, "limit", 50, "maximum number of entries to return")

	rootCmd.AddCommand(auditCmd)
}
