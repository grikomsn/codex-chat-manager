package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	deleteIDs  []string
	deleteJSON bool
	deleteYes  bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete --id ID [--id ID2 ...] [--yes]",
	Short: "Delete Codex sessions",
	Long: `Delete one or more Codex sessions permanently.

This is a destructive operation. Active sessions are deleted in place.

Examples:
  codex-chat-manager sessions delete --id abc123 --yes
  codex-chat-manager sessions delete --id abc123 --id def456 --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteYes {
			return fmt.Errorf("delete requires --yes to confirm")
		}
		store, err := resolveStore(codexHome)
		if err != nil {
			return err
		}
		plan, err := store.Delete(deleteIDs)
		if err != nil {
			if deleteJSON {
				_ = printJSON(cmd.OutOrStdout(), plan)
			}
			return err
		}
		if deleteJSON {
			return printJSON(cmd.OutOrStdout(), plan)
		}
		return printActionPlan(cmd.OutOrStdout(), plan)
	},
}

func init() {
	deleteCmd.Flags().StringArrayVarP(&deleteIDs, "id", "i", nil, "session ID to delete (required, can be specified multiple times)")
	deleteCmd.Flags().BoolVarP(&deleteJSON, "json", "j", false, "render as JSON")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	deleteCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(deleteCmd)
}
