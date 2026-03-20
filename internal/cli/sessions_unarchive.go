package cli

import (
	"github.com/spf13/cobra"
)

var (
	unarchiveIDs  []string
	unarchiveJSON bool
)

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive --id ID [--id ID2 ...]",
	Short: "Unarchive Codex sessions",
	Long: `Unarchive one or more Codex sessions by moving them back to the sessions directory.

Unarchiving restores previously archived sessions to active status.

Examples:
  codex-chat-manager sessions unarchive --id abc123
  codex-chat-manager sessions unarchive --id abc123 --id def456
  codex-chat-manager sessions unarchive --id abc123 --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveStore(codexHome)
		if err != nil {
			if unarchiveJSON {
				return printJSONCommandError(cmd, jsonErrorInventoryUnavailable, err, nil)
			}
			return err
		}
		plan, err := store.Unarchive(unarchiveIDs)
		if err != nil {
			if unarchiveJSON {
				return printJSONCommandError(cmd, actionFailureCode(plan, jsonErrorOperationFailed), err, actionPlanDetails(plan))
			}
			return err
		}
		if unarchiveJSON {
			return printJSON(cmd.OutOrStdout(), cmd, plan)
		}
		return printActionPlan(cmd.OutOrStdout(), plan)
	},
}

func init() {
	unarchiveCmd.Flags().StringArrayVarP(&unarchiveIDs, "id", "i", nil, "session ID to unarchive (required, can be specified multiple times)")
	unarchiveCmd.Flags().BoolVarP(&unarchiveJSON, "json", "j", false, "render as JSON")
	unarchiveCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(unarchiveCmd)
}
