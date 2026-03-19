package cli

import (
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage Codex sessions",
	Long: `Manage OpenAI Codex chat sessions through various subcommands.

Sessions can be listed, archived, unarchived, deleted, or resumed.
Use "codex-chat-manager sessions <command> --help" for more information about a command.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
}
