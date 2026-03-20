package cli

import (
	"github.com/spf13/cobra"
)

var (
	archiveIDs  []string
	archiveJSON bool
)

var archiveCmd = &cobra.Command{
	Use:   "archive --id ID [--id ID2 ...]",
	Short: "Archive Codex sessions",
	Long: `Archive one or more Codex sessions by moving them to the archived directory.

Archived sessions can later be unarchived to restore them to active status.

Examples:
  codex-chat-manager sessions archive --id abc123
  codex-chat-manager sessions archive --id abc123 --id def456
  codex-chat-manager sessions archive --id abc123 --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveStore(codexHome)
		if err != nil {
			if archiveJSON {
				return printJSONCommandError(cmd, jsonErrorInventoryUnavailable, err, nil)
			}
			return err
		}
		plan, err := store.Archive(archiveIDs)
		if err != nil {
			if archiveJSON {
				return printJSONCommandError(cmd, actionFailureCode(plan, jsonErrorOperationFailed), err, actionPlanDetails(plan))
			}
			return err
		}
		if archiveJSON {
			return printJSON(cmd.OutOrStdout(), cmd, plan)
		}
		return printActionPlan(cmd.OutOrStdout(), plan)
	},
}

func init() {
	archiveCmd.Flags().StringArrayVarP(&archiveIDs, "id", "i", nil, "session ID to archive (required, can be specified multiple times)")
	archiveCmd.Flags().BoolVarP(&archiveJSON, "json", "j", false, "render as JSON")
	archiveCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(archiveCmd)
}
