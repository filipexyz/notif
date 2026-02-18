package cmd

import (
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Manage NATS accounts",
	Long:  `Manage NATS account JWTs for multi-tenant isolation.`,
}

var rebuildAllOperatorSeed string

var accountsRebuildAllCmd = &cobra.Command{
	Use:   "rebuild-all",
	Short: "Rebuild all account JWTs with current (or new) operator key",
	Long: `Iterates all organizations and rebuilds their NATS account JWTs.
Used during OPERATOR_SEED rotation to re-sign all JWTs with the new key.

See docs/operator-seed-rotation.md for the full rotation procedure.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement when nats-accounts wish lands.
		// This stub exists to support the OPERATOR_SEED rotation runbook.
		//
		// Implementation will:
		// 1. Load all orgs from DB
		// 2. For each org, call RebuildAndPush with the operator key
		// 3. Audit log each rebuild as "jwt.push"
		// 4. Report summary
		out.Warn("rebuild-all is a stub — will be implemented with the nats-accounts wish")
		out.Info("See docs/operator-seed-rotation.md for the rotation procedure")
	},
}

var accountsVerifyAllCmd = &cobra.Command{
	Use:   "verify-all",
	Short: "Verify all account JWTs are signed by current operator",
	Long: `Checks every account JWT in NATS to verify it was signed by the current
operator key. Used after OPERATOR_SEED rotation to confirm all accounts are valid.

See docs/operator-seed-rotation.md for the full rotation procedure.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement when nats-accounts wish lands.
		// This stub exists to support the OPERATOR_SEED rotation runbook.
		//
		// Implementation will:
		// 1. Load all orgs from DB
		// 2. For each org, fetch the account JWT from NATS
		// 3. Verify the JWT signature against the current operator public key
		// 4. Report pass/fail per account
		out.Warn("verify-all is a stub — will be implemented with the nats-accounts wish")
		out.Info("See docs/operator-seed-rotation.md for the rotation procedure")
	},
}

func init() {
	accountsRebuildAllCmd.Flags().StringVar(&rebuildAllOperatorSeed, "operator-seed", "", "new operator seed (if rotating)")

	accountsCmd.AddCommand(accountsRebuildAllCmd)
	accountsCmd.AddCommand(accountsVerifyAllCmd)

	rootCmd.AddCommand(accountsCmd)
}
