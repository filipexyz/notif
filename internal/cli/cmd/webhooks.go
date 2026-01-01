package cmd

import (
	"strings"

	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Manage webhooks",
	Long:  `Create, list, update, and delete webhooks for HTTP event delivery.`,
}

var webhooksCreateURL string
var webhooksCreateTopics string

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook",
	Long: `Create a new webhook that will receive HTTP POST requests for matching events.

Examples:
  notif webhooks create --url https://example.com/webhook --topics "orders.*"
  notif webhooks create --url https://api.example.com/events --topics "orders.created,users.signup"`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		if webhooksCreateURL == "" {
			out.Error("--url is required")
			return
		}
		if webhooksCreateTopics == "" {
			out.Error("--topics is required")
			return
		}

		topics := strings.Split(webhooksCreateTopics, ",")
		for i := range topics {
			topics[i] = strings.TrimSpace(topics[i])
		}

		c := getClient()
		webhook, err := c.WebhookCreate(webhooksCreateURL, topics)
		if err != nil {
			out.Error("Failed to create webhook: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(webhook)
			return
		}

		out.Success("Webhook created")
		out.KeyValue("ID", webhook.ID)
		out.KeyValue("URL", webhook.URL)
		out.KeyValue("Topics", strings.Join(webhook.Topics, ", "))
		out.KeyValue("Secret", webhook.Secret)
		out.Warn("Save the secret - it won't be shown again!")
	},
}

var webhooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all webhooks",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.WebhookList()
		if err != nil {
			out.Error("Failed to list webhooks: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No webhooks configured")
			return
		}

		out.Header("Webhooks")
		out.Divider()

		for _, wh := range result.Webhooks {
			status := "enabled"
			if !wh.Enabled {
				status = "disabled"
			}
			out.Info("%s (%s)", wh.ID, status)
			out.KeyValue("URL", wh.URL)
			out.KeyValue("Topics", strings.Join(wh.Topics, ", "))
			out.KeyValue("Created", wh.CreatedAt)
			out.Divider()
		}
	},
}

var webhooksGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get webhook details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		webhook, err := c.WebhookGet(args[0])
		if err != nil {
			out.Error("Failed to get webhook: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(webhook)
			return
		}

		out.Header("Webhook")
		out.KeyValue("ID", webhook.ID)
		out.KeyValue("URL", webhook.URL)
		out.KeyValue("Topics", strings.Join(webhook.Topics, ", "))
		out.KeyValue("Enabled", boolToStr(webhook.Enabled))
		out.KeyValue("Environment", webhook.Environment)
		out.KeyValue("Created", webhook.CreatedAt)
	},
}

var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a webhook",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		if err := c.WebhookDelete(args[0]); err != nil {
			out.Error("Failed to delete webhook: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(map[string]string{"status": "deleted"})
			return
		}

		out.Success("Webhook deleted")
	},
}

var webhooksEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a webhook",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		enabled := true
		c := getClient()
		_, err := c.WebhookUpdate(args[0], client.UpdateWebhookRequest{Enabled: &enabled})
		if err != nil {
			out.Error("Failed to enable webhook: %v", err)
			return
		}

		out.Success("Webhook enabled")
	},
}

var webhooksDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a webhook",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		enabled := false
		c := getClient()
		_, err := c.WebhookUpdate(args[0], client.UpdateWebhookRequest{Enabled: &enabled})
		if err != nil {
			out.Error("Failed to disable webhook: %v", err)
			return
		}

		out.Success("Webhook disabled")
	},
}

var webhooksDeliveriesCmd = &cobra.Command{
	Use:   "deliveries <id>",
	Short: "List recent deliveries for a webhook",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.WebhookDeliveries(args[0])
		if err != nil {
			out.Error("Failed to get deliveries: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No deliveries yet")
			return
		}

		out.Header("Recent Deliveries")
		out.Divider()

		for _, d := range result.Deliveries {
			statusIcon := "✓"
			if d.Status == "failed" {
				statusIcon = "✗"
			} else if d.Status == "pending" {
				statusIcon = "○"
			}
			out.Info("%s %s - %s", statusIcon, d.EventID, d.Topic)
			out.KeyValue("Status", d.Status)
			if d.ResponseStatus != nil {
				out.KeyValue("HTTP", string(rune(*d.ResponseStatus)))
			}
			if d.Error != nil && *d.Error != "" {
				out.KeyValue("Error", *d.Error)
			}
			out.Divider()
		}
	},
}

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func init() {
	webhooksCreateCmd.Flags().StringVar(&webhooksCreateURL, "url", "", "webhook URL")
	webhooksCreateCmd.Flags().StringVar(&webhooksCreateTopics, "topics", "", "comma-separated topic patterns")

	webhooksCmd.AddCommand(webhooksCreateCmd)
	webhooksCmd.AddCommand(webhooksListCmd)
	webhooksCmd.AddCommand(webhooksGetCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
	webhooksCmd.AddCommand(webhooksEnableCmd)
	webhooksCmd.AddCommand(webhooksDisableCmd)
	webhooksCmd.AddCommand(webhooksDeliveriesCmd)

	rootCmd.AddCommand(webhooksCmd)
}
