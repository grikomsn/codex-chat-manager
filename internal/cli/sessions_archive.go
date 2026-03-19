package cli

import (
	"fmt"

	"github.com/grikomsn/codex-chat-manager/internal/session"
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
		if len(archiveIDs) == 0 {
			return fmt.Errorf("archive requires at least one --id")
		}
		cfg, err := session.ResolveConfig(codexHome)
		if err != nil {
			return err
		}
		store := session.NewStore(cfg)
		plan, err := store.Archive(archiveIDs)
		if err != nil {
			return err
		}
		if archiveJSON {
			return printJSON(cmd.OutOrStdout(), plan)
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
