package cli

import (
	"fmt"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
)

var (
	deleteIDs  []string
	deleteJSON bool
	deleteYes  bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete --id ID [--id ID2 ...] [--yes]",
	Short: "Delete archived Codex sessions",
	Long: `Delete one or more archived Codex sessions permanently.

This is a destructive operation. Active sessions are protected and cannot be deleted through normal flows.

Examples:
  codex-chat-manager sessions delete --id abc123 --yes
  codex-chat-manager sessions delete --id abc123 --id def456 --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteYes {
			err := fmt.Errorf("delete requires --yes to confirm")
			if deleteJSON {
				return printJSONCommandError(cmd, jsonErrorInvalidRequest, err, map[string]string{
					"required_flag": "yes",
				})
			}
			return err
		}
		return runActionCommand(cmd, deleteJSON, deleteIDs, (*session.Store).Delete, deleteActionFailureCode)
	},
}

func init() {
	deleteCmd.Flags().StringArrayVarP(&deleteIDs, "id", "i", nil, "session ID to delete (required, can be specified multiple times)")
	deleteCmd.Flags().BoolVarP(&deleteJSON, "json", "j", false, "render as JSON")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	deleteCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(deleteCmd)
}
