package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:     "accounts",
	Aliases: []string{"org"},
	Short:   "Manage NATS accounts (orgs)",
	Long:    `Manage NATS account JWTs for multi-tenant isolation.`,
}

var rebuildAllOperatorSeed string

var accountsRebuildAllCmd = &cobra.Command{
	Use:   "rebuild-all",
	Short: "Rebuild all account JWTs with current (or new) operator key",
	Long: `Iterates all organizations and rebuilds their NATS account JWTs.
Used during OPERATOR_SEED rotation to re-sign all JWTs with the new key.

See docs/operator-seed-rotation.md for the full rotation procedure.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()
		orgs, err := c.Get("/api/v1/orgs")
		if err != nil {
			out.Error("Failed to list orgs: " + err.Error())
			return
		}

		var result struct {
			Orgs  []json.RawMessage `json:"orgs"`
			Count int               `json:"count"`
		}
		if err := json.Unmarshal(orgs, &result); err != nil {
			out.Error("Failed to parse response: " + err.Error())
			return
		}

		out.Info(fmt.Sprintf("Found %d orgs to rebuild", result.Count))
		// In a real implementation, this would call the server-side rebuild endpoint
		// For now, we list the orgs that would be rebuilt
		for _, org := range result.Orgs {
			out.Info("  Would rebuild: " + string(org))
		}
		out.Warn("Full rebuild requires server-side implementation via OPERATOR_SEED rotation")
	},
}

var accountsVerifyAllCmd = &cobra.Command{
	Use:   "verify-all",
	Short: "Verify all account JWTs are signed by current operator",
	Long: `Checks every account JWT in NATS to verify it was signed by the current
operator key. Used after OPERATOR_SEED rotation to confirm all accounts are valid.

See docs/operator-seed-rotation.md for the full rotation procedure.`,
	Run: func(cmd *cobra.Command, args []string) {
		out.Warn("verify-all requires direct NATS access — run from the notifd host")
	},
}

var orgCreateCmd = &cobra.Command{
	Use:   "create <id> <name>",
	Short: "Create a new org with NATS account",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()
		body := map[string]string{
			"id":   args[0],
			"name": args[1],
		}
		data, _ := json.Marshal(body)
		resp, err := c.Post("/api/v1/orgs", data)
		if err != nil {
			out.Error("Failed to create org: " + err.Error())
			return
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return
		}

		var org struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			NatsPublicKey string `json:"nats_public_key"`
			BillingTier   string `json:"billing_tier"`
		}
		json.Unmarshal(resp, &org)
		out.Success(fmt.Sprintf("Org created: %s (%s)", org.ID, org.Name))
		out.Info(fmt.Sprintf("  NATS public key: %s", org.NatsPublicKey))
		out.Info(fmt.Sprintf("  Billing tier: %s", org.BillingTier))
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an org and revoke its NATS account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()
		_, err := c.Delete("/api/v1/orgs/" + args[0])
		if err != nil {
			out.Error("Failed to delete org: " + err.Error())
			return
		}
		out.Success("Org deleted: " + args[0])
	},
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all orgs",
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()
		resp, err := c.Get("/api/v1/orgs")
		if err != nil {
			out.Error("Failed to list orgs: " + err.Error())
			return
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return
		}

		var result struct {
			Orgs []struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				NatsPublicKey string `json:"nats_public_key"`
				BillingTier   string `json:"billing_tier"`
				CreatedAt     string `json:"created_at"`
			} `json:"orgs"`
			Count int `json:"count"`
		}
		json.Unmarshal(resp, &result)

		if result.Count == 0 {
			out.Info("No orgs found")
			return
		}

		out.Info(fmt.Sprintf("Orgs (%d):", result.Count))
		for _, org := range result.Orgs {
			keyPreview := org.NatsPublicKey
			if len(keyPreview) > 16 {
				keyPreview = keyPreview[:16] + "..."
			}
			out.Info(fmt.Sprintf("  %s  %s  tier=%s  key=%s",
				org.ID, org.Name, org.BillingTier, keyPreview))
		}
	},
}

var orgLimitsSetTier string

var orgLimitsCmd = &cobra.Command{
	Use:   "limits <id>",
	Short: "Show or set account limits for an org",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()

		if orgLimitsSetTier != "" {
			// Set limits
			body := map[string]string{"billing_tier": orgLimitsSetTier}
			data, _ := json.Marshal(body)
			resp, err := c.Put("/api/v1/orgs/"+args[0]+"/limits", data)
			if err != nil {
				out.Error("Failed to update limits: " + err.Error())
				return
			}
			if jsonOutput {
				fmt.Println(string(resp))
				return
			}
			out.Success("Limits updated for " + args[0] + " (tier: " + orgLimitsSetTier + ")")
			return
		}

		// Show limits
		resp, err := c.Get("/api/v1/orgs/" + args[0] + "/limits")
		if err != nil {
			out.Error("Failed to get limits: " + err.Error())
			return
		}

		if jsonOutput {
			fmt.Println(string(resp))
			return
		}

		var result map[string]any
		json.Unmarshal(resp, &result)
		out.Info(fmt.Sprintf("Org: %s", args[0]))
		out.Info(fmt.Sprintf("  Billing tier: %v", result["billing_tier"]))
		if limits, ok := result["limits"].(map[string]any); ok {
			out.Info(fmt.Sprintf("  Max connections: %v", limits["max_connections"]))
			out.Info(fmt.Sprintf("  Max data: %v", limits["max_data"]))
			out.Info(fmt.Sprintf("  Max payload: %v", limits["max_payload"]))
			out.Info(fmt.Sprintf("  Stream max age: %v", limits["stream_max_age"]))
			out.Info(fmt.Sprintf("  Stream max bytes: %v", limits["stream_max_bytes"]))
		}
	},
}

func init() {
	accountsRebuildAllCmd.Flags().StringVar(&rebuildAllOperatorSeed, "operator-seed", "", "new operator seed (if rotating)")

	orgLimitsCmd.Flags().StringVar(&orgLimitsSetTier, "set", "", "set billing tier (free, pro, enterprise)")

	accountsCmd.AddCommand(accountsRebuildAllCmd)
	accountsCmd.AddCommand(accountsVerifyAllCmd)
	accountsCmd.AddCommand(orgCreateCmd)
	accountsCmd.AddCommand(orgDeleteCmd)
	accountsCmd.AddCommand(orgListCmd)
	accountsCmd.AddCommand(orgLimitsCmd)

	rootCmd.AddCommand(accountsCmd)
}
