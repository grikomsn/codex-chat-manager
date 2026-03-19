package cli

import (
	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/grikomsn/codex-chat-manager/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long:  `Launch the interactive terminal user interface for browsing and managing Codex sessions.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := session.ResolveConfig(codexHome)
		if err != nil {
			return err
		}
		return tui.Run(cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
