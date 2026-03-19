package cli

import (
	"fmt"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
)

var (
	resumeID string
)

var resumeCmd = &cobra.Command{
	Use:   "resume --id ID",
	Short: "Resume a Codex session",
	Long: `Resume an active Codex session in a new terminal.

Only active (non-archived) sessions can be resumed.

Examples:
  codex-chat-manager sessions resume --id abc123`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if resumeID == "" {
			return fmt.Errorf("resume requires --id")
		}
		cfg, err := session.ResolveConfig(codexHome)
		if err != nil {
			return err
		}
		store := session.NewStore(cfg)
		return store.Resume(nil, resumeID)
	},
}

func init() {
	resumeCmd.Flags().StringVarP(&resumeID, "id", "i", "", "session ID to resume (required)")
	resumeCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(resumeCmd)
}
