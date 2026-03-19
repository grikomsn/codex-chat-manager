package cli

import (
	"fmt"

	"github.com/grikomsn/codex-chat-manager/internal/session"
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
		if len(unarchiveIDs) == 0 {
			return fmt.Errorf("unarchive requires at least one --id")
		}
		cfg, err := session.ResolveConfig(codexHome)
		if err != nil {
			return err
		}
		store := session.NewStore(cfg)
		plan, err := store.Unarchive(unarchiveIDs)
		if err != nil {
			return err
		}
		if unarchiveJSON {
			return printJSON(cmd.OutOrStdout(), plan)
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
